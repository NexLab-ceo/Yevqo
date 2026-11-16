package agent

import (
 "net"
 "fmt"
 "strconv"
 "time"
)

// waitForTCP polls the app's local listener until it is ready or times out.
func waitForTCP(port int, timeout time.Duration) error {
 deadline:=time.Now().Add(timeout)
 address:=net.JoinHostPort("127.0.0.1",strconv.Itoa(port))
 for time.Now().Before(deadline) {
  conn,err:=net.DialTimeout("tcp",address,200*time.Millisecond)
  if err==nil { _=conn.Close(); return nil }
  time.Sleep(100*time.Millisecond)
 }
 return fmt.Errorf("TCP listener %s did not become ready within %s",address,timeout)
}
