package agent

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func reserveTestPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil { t.Fatal(err) }
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return port
}

func TestDeploymentProxySwitchesBackendAtomically(t *testing.T) {
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "first") }))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "second") }))
	defer second.Close()

	firstPort := first.Listener.Addr().(*net.TCPAddr).Port
	secondPort := second.Listener.Addr().(*net.TCPAddr).Port
	publicPort := reserveTestPort(t)
	proxy, err := newDeploymentProxy(publicPort, firstPort)
	if err != nil { t.Fatal(err) }
	defer proxy.close()

	request := func(want string) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", publicPort))
			if err == nil {
				body, readErr := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if readErr == nil && resp.StatusCode == http.StatusOK && strings.TrimSpace(string(body)) == want { return }
			}
			time.Sleep(25 * time.Millisecond)
		}
		t.Fatalf("proxy did not serve backend %q", want)
	}

	request("first")
	if err := proxy.switchBackend(secondPort); err != nil { t.Fatal(err) }
	request("second")
}
