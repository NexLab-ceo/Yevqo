package agent

import (
 "fmt"
 "os"
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
  if !validSiteID(r.SiteID)||!verifyProcessRecord(r){_ = m.registry.Delete(r.SiteID);continue}
  m.procs[r.SiteID]=&Process{SiteID:r.SiteID,PID:r.PID,Port:r.Port,Runtime:r.Runtime,LogPath:r.LogPath}
 }
}
func killProcessGroup(pid,pgid int) error {
 if pid<=0||pgid<=0||pid!=pgid{return fmt.Errorf("invalid managed process group")}
 // Re-check the identity immediately before signaling.
 if !isAlive(pid){return fmt.Errorf("process %d is not running",pid)}
 if err:=syscall.Kill(-pgid,syscall.SIGTERM);err!=nil{return fmt.Errorf("signal process group SIGTERM: %w",err)}
 deadline:=time.Now().Add(2*time.Second)
 for time.Now().Before(deadline){if !isAlive(pid){return nil};time.Sleep(100*time.Millisecond)}
 if isAlive(pid){if err:=syscall.Kill(-pgid,syscall.SIGKILL);err!=nil{return fmt.Errorf("signal process group SIGKILL: %w",err)}}
 return nil
}
