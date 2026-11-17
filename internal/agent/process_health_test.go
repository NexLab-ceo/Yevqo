package agent

import (
 "context"
 "net/http"
 "net/http/httptest"
 "strconv"
 "strings"
 "testing"
 "time"
)

func testServerPort(t *testing.T, server *httptest.Server) int {
 t.Helper()
 parts := strings.Split(server.Listener.Addr().String(), ":")
 port, err := strconv.Atoi(parts[len(parts)-1])
 if err != nil { t.Fatal(err) }
 return port
}

func TestWaitForHTTPHealthCheck(t *testing.T) {
 server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  if r.URL.Path != "/health" { http.NotFound(w, r); return }
  w.WriteHeader(http.StatusNoContent)
 }))
 defer server.Close()
 if err := waitForHTTP(testServerPort(t, server), "/health", time.Second); err != nil { t.Fatalf("expected healthy response: %v", err) }
}

func TestWaitForHTTPRejectsUnhealthyStatusAndRedirects(t *testing.T) {
 target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
 defer target.Close()
 redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
 defer redirect.Close()
 if err := waitForHTTP(testServerPort(t, redirect), "/health", 350*time.Millisecond); err == nil {
  t.Fatal("expected redirect not to count as a healthy response")
 }
 unhealthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
 defer unhealthy.Close()
 if err := waitForHTTP(testServerPort(t, unhealthy), "/health", 350*time.Millisecond); err == nil {
  t.Fatal("expected HTTP 503 to fail health check")
 }
}

func TestValidHealthCheckPath(t *testing.T) {
 for _, path := range []string{"", "/", "/health", "/api/ready?deep=1"} {
  if !validHealthCheckPath(path) { t.Errorf("expected path %q to be valid", path) }
 }
 for _, path := range []string{"health", "//example.com", "/../secret", "/bad\rpath", strings.Repeat("a", 256)} {
  if validHealthCheckPath(path) { t.Errorf("expected path %q to be rejected", path) }
 }
}


func TestWaitForHTTPHonorsCancellation(t *testing.T) {
 ctx, cancel := context.WithCancel(context.Background())
 cancel()
 err := waitForHTTPContext(ctx, 65000, "/health", time.Second)
 if err != context.Canceled { t.Fatalf("expected cancellation, got %v", err) }
}
