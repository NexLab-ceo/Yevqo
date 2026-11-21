package agent

import (
 "net"
 "os"
 "path/filepath"
 "strings"
 "strconv"
 "testing"
)

func TestCheckLocalPortReportsListeningAndFreePorts(t *testing.T) {
 listener,err:=net.Listen("tcp","127.0.0.1:0");if err!=nil{t.Fatal(err)}
 port:=listener.Addr().(*net.TCPAddr).Port
 listening,err:=checkLocalPort(port);if err!=nil{t.Fatal(err)}
 if !listening{t.Fatal("expected active loopback listener to be detected")}
 _=listener.Close()
 listening,err=checkLocalPort(port);if err!=nil{t.Fatal(err)}
 if listening{t.Fatal("expected closed port to be reported as free")}
}

func TestCheckLocalPortRejectsInvalidPort(t *testing.T) {
 for _,port:=range []int{-1,0,65536}{
  if _,err:=checkLocalPort(port);err==nil{t.Errorf("expected port %d to be rejected",port)}
 }
}

func TestDeviceDiagnosticsReturnsPackageInventoryAndUptime(t *testing.T) {
 got:=collectDeviceDiagnostics()
 if got.Packages==nil{t.Fatal("package inventory must always be present")}
 for _,name:=range []string{"php","node","git","cloudflared"}{
  if _,ok:=got.Packages[name];!ok{t.Errorf("missing package %q",name)}
 }
 if got.AgentUptimeSeconds<0||got.DeviceUptimeSeconds<0{t.Fatal("uptime values must not be negative")}
 if got.ListeningPorts==nil{t.Fatal("listening ports should be an empty or populated array, not null")}
}

func TestOpenAgentLogCreatesPrivateFile(t *testing.T) {
 root:=t.TempDir()
 file,err:=OpenAgentLog(root);if err!=nil{t.Fatal(err)}
 if _,err:=file.WriteString("agent started\n");err!=nil{t.Fatal(err)}
 if err:=file.Close();err!=nil{t.Fatal(err)}
 info,err:=os.Stat(filepath.Join(root,"logs","agent.log"));if err!=nil{t.Fatal(err)}
 if info.Mode().Perm()!=0600{t.Fatalf("agent.log permissions = %o, want 600",info.Mode().Perm())}
}

func TestOpenAgentLogRejectsSymlink(t *testing.T) {
 root:=t.TempDir()
 target:=filepath.Join(root,"outside.log")
 if err:=os.WriteFile(target,[]byte("do not touch"),0600);err!=nil{t.Fatal(err)}
 logs:=filepath.Join(root,"logs");if err:=os.Mkdir(logs,0700);err!=nil{t.Fatal(err)}
 if err:=os.Symlink(target,filepath.Join(logs,"agent.log"));err!=nil{t.Fatal(err)}
 if _,err:=OpenAgentLog(root);err==nil{t.Fatal("expected symlink log to be rejected")}
 data,err:=os.ReadFile(target);if err!=nil{t.Fatal(err)}
 if string(data)!="do not touch"{t.Fatal("symlink target was modified")}
}

func TestReadAgentLogsReturnsNewestLinesFromAgentLog(t *testing.T) {
 root:=t.TempDir()
 file,err:=OpenAgentLog(root);if err!=nil{t.Fatal(err)}
 for i:=0;i<205;i++{_,_=file.WriteString("line "+strconv.Itoa(i)+"\n")}
 _=file.Close()
 got:=readAgentLogs(root)
 if strings.Contains(got,"line 0\n")||!strings.Contains(got,"line 204"){t.Fatalf("unexpected log tail: %s",got[:min(len(got),200)])}
}
