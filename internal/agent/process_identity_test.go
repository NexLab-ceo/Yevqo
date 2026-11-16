package agent

import (
 "os/exec"
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
