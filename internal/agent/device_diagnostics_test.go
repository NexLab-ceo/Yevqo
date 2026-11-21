package agent

import (
 "net"
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
