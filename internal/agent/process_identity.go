package agent

import (
 "fmt"
 "os"
 "path/filepath"
 "strconv"
 "strings"
 "syscall"
 "time"
)

// processStartToken reads Linux /proc starttime (clock ticks since boot), protecting
// against PID reuse after an Agent restart.
func processStartToken(pid int) (string,error) {
 b,err:=os.ReadFile(fmt.Sprintf("/proc/%d/stat",pid)); if err!=nil{return "",err}
 s:=string(b); close:=strings.LastIndex(s,")"); if close<0{return "",fmt.Errorf("malformed proc stat")}
 fields:=strings.Fields(s[close+2:]) // field 3 onward; starttime is field 22 => index 19
 if len(fields)<=19{return "",fmt.Errorf("proc stat has no starttime")}
 if _,err:=strconv.ParseUint(fields[19],10,64);err!=nil{return "",err}
 return fields[19],nil
}
func verifyProcessRecord(r ProcessRecord) bool {
 if r.PID<=0 || r.ProcessGroup<=0 || r.StartToken=="" || !isAlive(r.PID){return false}
 token,err:=processStartToken(r.PID);return err==nil && token==r.StartToken
}
func (m *ProcessManager) restoreProcesses() {
 for _,r:=range m.registry.List(){
  if !validSiteID(r.SiteID) { _ = m.registry.Delete(r.SiteID); continue }
  if !verifyProcessRecord(r) {
   _ = m.registry.Delete(r.SiteID)
   if shouldRecoverAfterAgentRestart(r.RestartPolicy) { go m.recoverSite(r) }
   continue
  }
  if r.BackendPort > 0 {
   if err := m.ensureDeploymentProxy(r.SiteID, r.Port, r.BackendPort); err != nil {
    m.emitProcessLog(r.SiteID, fmt.Sprintf("could not restore stable deployment proxy: %v", err))
   }
  }
  m.procs[r.SiteID]=&Process{SiteID:r.SiteID,PID:r.PID,Port:r.Port,Runtime:r.Runtime,LogPath:r.LogPath}
  go m.watchRestoredProcess(r)
  if manifest,found:=readProcessYevqoManifest(m.DataRoot,r);found&&manifest.HealthcheckPath!=""{
   interval:=manifest.HealthcheckInterval;if interval<=0{interval=30*time.Second}
   go m.monitorSiteHealth(r.SiteID,r.PID,r.StartToken,r.Port,manifest.HealthcheckPath,interval)
  }
 }
}

// watchRestoredProcess monitors a process adopted after Agent restart. Such a
// process is not a child, so cmd.Wait cannot reap it; poll its start identity.
func (m *ProcessManager) watchRestoredProcess(record ProcessRecord) {
 ticker := time.NewTicker(2 * time.Second)
 defer ticker.Stop()
 for range ticker.C {
  if verifyProcessRecord(record) { continue }
  current, ok := m.registry.Get(record.SiteID)
  if !ok || current.PID != record.PID || current.StartToken != record.StartToken { return }
  m.mu.Lock()
  managed, exists := m.procs[record.SiteID]
  if exists && managed.PID == record.PID { delete(m.procs, record.SiteID) }
  m.mu.Unlock()
  _ = m.registry.Delete(record.SiteID)
  m.restartMu.Lock()
  if m.restartAttempts == nil { m.restartAttempts = make(map[string]int) }
  if m.intentionalStops == nil { m.intentionalStops = make(map[string]bool) }
  intentional := m.intentionalStops[record.SiteID]
  m.restartMu.Unlock()
  if !intentional && shouldRecoverAfterAgentRestart(record.RestartPolicy) {
   m.emitProcessStatus(record.SiteID, "error", 0)
   go m.recoverSite(record)
  } else if !intentional {
   _ = m.closeDeploymentProxy(record.SiteID)
  }
  return
 }
}
// processGroupAlive checks every member of the group, not just its leader.
// Linux may retain killed orphan descendants as zombies briefly; they cannot
// execute and must not make a successful shutdown look like a live service.
func processGroupAlive(pgid int) bool {
 if pgid <= 0 { return false }
 entries, err := os.ReadDir("/proc")
 if err == nil {
  for _, entry := range entries {
   if _, parseErr := strconv.Atoi(entry.Name()); parseErr != nil { continue }
   data, readErr := os.ReadFile("/proc/" + entry.Name() + "/stat")
   if readErr != nil { continue }
   raw := string(data)
   end := strings.LastIndex(raw, ")")
   if end < 0 { continue }
   fields := strings.Fields(raw[end+2:]) // state, ppid, pgrp, ...
   if len(fields) < 3 || fields[0] == "Z" || fields[0] == "X" { continue }
   group, parseErr := strconv.Atoi(fields[2])
   if parseErr == nil && group == pgid { return true }
  }
  return false
 }
 // Fallback for non-Linux environments where /proc is unavailable.
 err = syscall.Kill(-pgid, 0)
 return err == nil || err == syscall.EPERM
}

func killProcessGroup(pid, pgid int) error {
 if pid <= 0 || pgid <= 0 || pid != pgid { return fmt.Errorf("invalid managed process group") }
 // Re-check the identity immediately before signaling; never trust a persisted PID.
 if !isAlive(pid) { return fmt.Errorf("process %d is not running", pid) }
 if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
  return fmt.Errorf("signal process group SIGTERM: %w", err)
 }
 deadline := time.Now().Add(2 * time.Second)
 for time.Now().Before(deadline) {
  if !processGroupAlive(pgid) { return nil }
  time.Sleep(100 * time.Millisecond)
 }
 // SIGKILL all remaining descendants even if the group leader already exited.
 if processGroupAlive(pgid) {
  if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
   return fmt.Errorf("signal process group SIGKILL: %w", err)
  }
 }
 deadline = time.Now().Add(time.Second)
 for time.Now().Before(deadline) {
  if !processGroupAlive(pgid) { return nil }
  time.Sleep(50 * time.Millisecond)
 }
 if processGroupAlive(pgid) { return fmt.Errorf("process group %d did not exit after SIGKILL", pgid) }
 return nil
}

func readProcessYevqoManifest(dataRoot string,record ProcessRecord)(yevqoManifest,bool){
 siteRoot,err:=safeSiteDirectory(dataRoot,record.SiteID);if err!=nil{return yevqoManifest{},false}
 current,err:=filepath.Abs(record.WorkingDir);if err!=nil{return yevqoManifest{},false}
 root,err:=filepath.EvalSymlinks(siteRoot);if err!=nil{return yevqoManifest{},false}
 for{
  resolved,e:=filepath.EvalSymlinks(current);if e==nil{
   rel,re:=filepath.Rel(root,resolved);if re==nil&&rel!=".."&&!strings.HasPrefix(rel,".."+string(filepath.Separator)){
    if m,found,readErr:=readYevqoManifest(resolved);readErr==nil&&found{return m,true}
   }
  }
  if current==root||filepath.Dir(current)==current{return yevqoManifest{},false}
  parent:=filepath.Dir(current);rel,e:=filepath.Rel(root,parent);if e!=nil||rel==".."||strings.HasPrefix(rel,".."+string(filepath.Separator)){return yevqoManifest{},false};current=parent
 }
}
