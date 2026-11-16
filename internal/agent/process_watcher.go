package agent

import (
 "time"
 "net"
 "strconv"
)

// watchProcesses reconciles the in-memory view with the durable registry.
// It detects unexpected exits and removes stale entries (no PID guessing).
func (m *ProcessManager) watchProcesses() {
 ticker:=time.NewTicker(2*time.Second); defer ticker.Stop()
 for range ticker.C {
  if m.registry==nil { return }
  for _,record:=range m.registry.List() {
   if verifyProcessRecord(record) { continue }
   m.mu.Lock()
   delete(m.procs,record.SiteID)
   m.mu.Unlock()
   _=m.registry.Delete(record.SiteID)
  }
 }
}

// IsSiteHealthy reports whether a managed process identity is valid and its
// configured TCP port accepts connections on localhost.
func (m *ProcessManager) IsSiteHealthy(siteID string) bool {
 if !validSiteID(siteID)||m.registry==nil{return false}
 r,ok:=m.registry.Get(siteID);if !ok||!verifyProcessRecord(r){return false}
 conn,err:=net.DialTimeout("tcp",net.JoinHostPort("127.0.0.1",strconv.Itoa(r.Port)),500*time.Millisecond)
 if err!=nil{return false};_ = conn.Close();return true
}
