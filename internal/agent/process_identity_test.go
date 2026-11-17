package agent

import (
 "os/exec"
 "path/filepath"
 "syscall"
 "testing"
 "time"
)

func TestKillProcessGroupStopsLeader(t *testing.T) {
 cmd := exec.Command("sleep", "30")
 cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid:true}
 if err:=cmd.Start();err!=nil { t.Skipf("cannot start process group: %v",err) }
 pid:=cmd.Process.Pid
 token,err:=processStartToken(pid);if err!=nil { _=cmd.Process.Kill();t.Fatal(err) }
 if !verifyProcessRecord(ProcessRecord{PID:pid,ProcessGroup:pid,StartToken:token}) { _=cmd.Process.Kill();t.Fatal("new process identity did not verify") }
 if err:=killProcessGroup(pid,pid);err!=nil { _=cmd.Process.Kill();t.Fatalf("killProcessGroup: %v",err) }
 done:=make(chan error,1);go func(){done<-cmd.Wait()}()
 select {case <-done:case <-time.After(2*time.Second):t.Fatal("leader was not reaped")}
 if processGroupAlive(pid) { t.Fatalf("process group %d still exists after stop",pid) }
}


func TestRestoreProcessesAdoptsExistingProcessAfterAgentRestart(t *testing.T) {
 root := t.TempDir()
 cmd := exec.Command("sleep", "30")
 cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid:true}
 if err := cmd.Start(); err != nil { t.Skipf("cannot start process group: %v", err) }
 pid := cmd.Process.Pid
 token, err := processStartToken(pid)
 if err != nil { _ = cmd.Process.Kill(); _ = cmd.Wait(); t.Fatal(err) }
 registry, err := NewProcessRegistry(root)
 if err != nil { _ = cmd.Process.Kill(); _ = cmd.Wait(); t.Fatal(err) }
 record := ProcessRecord{SiteID:"adopted", PID:pid, ProcessGroup:pid, Port:3000, Runtime:"static", WorkingDir:root, LogPath:filepath.Join(root,"logs","adopted.log"), StartedAt:time.Now(), StartToken:token, RunPath:filepath.Join("sites","adopted"), RestartPolicy:"never"}
 if err := registry.Save(record); err != nil { _ = cmd.Process.Kill(); _ = cmd.Wait(); t.Fatal(err) }
 manager := &ProcessManager{DataRoot:root, registry:registry, procs:map[string]*Process{}, restartAttempts:map[string]int{}, intentionalStops:map[string]bool{}}
 manager.restoreProcesses()
 adopted, ok := manager.Get("adopted")
 if !ok || adopted.PID != pid { _ = cmd.Process.Kill(); _ = cmd.Wait(); t.Fatalf("process was not adopted after Agent restart: %#v", adopted) }
 if err := manager.StopSite("adopted"); err != nil { _ = cmd.Process.Kill(); _ = cmd.Wait(); t.Fatalf("stop adopted process: %v", err) }
 _ = cmd.Wait()
 if _, ok := registry.Get("adopted"); ok { t.Fatal("adopted process registry record remains after stop") }
}
