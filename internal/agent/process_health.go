package agent

import (
 "net"
 "net/http"
 "fmt"
 "strconv"
 "strings"
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


// validHealthCheckPath restricts probes to a local origin-relative URL path.
func validHealthCheckPath(path string) bool {
 if path == "" { return true }
 if len(path) > 255 || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") ||
  strings.Contains(path, "..") || strings.ContainsAny(path, "\r\n\x00") { return false }
 return true
}

// waitForHTTP requires a successful 2xx response from the site's local listener.
// Redirects are not followed, preventing a site from redirecting the probe off-device.
func waitForHTTP(port int, path string, timeout time.Duration) error {
 if !validHealthCheckPath(path) { return fmt.Errorf("invalid HTTP health check path") }
 if path == "" { return nil }
 client := &http.Client{Timeout: 800 * time.Millisecond, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
 endpoint := "http://127.0.0.1:" + strconv.Itoa(port) + path
 deadline := time.Now().Add(timeout)
 var lastErr error
 for time.Now().Before(deadline) {
  req, _ := http.NewRequest(http.MethodGet, endpoint, nil)
  response, err := client.Do(req)
  if err == nil {
   response.Body.Close()
   if response.StatusCode >= 200 && response.StatusCode < 300 { return nil }
   lastErr = fmt.Errorf("HTTP health check returned status %d", response.StatusCode)
  } else { lastErr = err }
  time.Sleep(150 * time.Millisecond)
 }
 if lastErr == nil { lastErr = fmt.Errorf("no HTTP response") }
 return fmt.Errorf("HTTP health check %s did not become healthy within %s: %w", endpoint, timeout, lastErr)
}
