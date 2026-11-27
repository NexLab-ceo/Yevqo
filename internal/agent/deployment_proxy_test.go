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

func deploymentProxyTestURL(proxy *deploymentProxy) string {
	return fmt.Sprintf("http://127.0.0.1:%d/", proxy.listener.Addr().(*net.TCPAddr).Port)
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
	previous, err := proxy.switchBackendAndGetPrevious(secondPort)
	if err != nil { t.Fatal(err) }
	if err := waitForDeploymentBackendDrain(previous, time.Second); err != nil { t.Fatalf("drain previous backend: %v", err) }
	request("second")
}

func TestDeploymentProxyDrainWaitsForInflightRequests(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		_, _ = io.WriteString(w, "first")
	}))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "second") }))
	defer second.Close()

	firstPort := first.Listener.Addr().(*net.TCPAddr).Port
	secondPort := second.Listener.Addr().(*net.TCPAddr).Port
	proxy, err := newDeploymentProxy(reserveTestPort(t), firstPort)
	if err != nil { t.Fatal(err) }
	defer proxy.close()

	responseDone := make(chan error, 1)
	go func() {
		resp, requestErr := http.Get(deploymentProxyTestURL(proxy))
		if requestErr != nil { responseDone <- requestErr; return }
		_, readErr := io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		responseDone <- readErr
	}()
	select {
	case <-entered:
	case <-time.After(2*time.Second): t.Fatal("request never reached the old backend")
	}

	previous, err := proxy.switchBackendAndGetPrevious(secondPort)
	if err != nil { t.Fatal(err) }
	drained := make(chan error, 1)
	go func() { drained <- waitForDeploymentBackendDrain(previous, time.Second) }()
	select {
	case err := <-drained:
		t.Fatalf("drain completed while an old request was still in flight: %v", err)
	case <-time.After(100*time.Millisecond):
	}
	close(release)
	if err := <-responseDone; err != nil { t.Fatalf("old in-flight request failed: %v", err) }
	if err := <-drained; err != nil { t.Fatalf("old backend did not drain after request completed: %v", err) }
}

func TestDeploymentProxyPreservesPublicHostAndForwardedProtocol(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Host+"|"+r.Header.Get("X-Forwarded-Host")+"|"+r.Header.Get("X-Forwarded-Proto"))
	}))
	defer backend.Close()
	backendPort := backend.Listener.Addr().(*net.TCPAddr).Port
	proxy, err := newDeploymentProxy(reserveTestPort(t), backendPort)
	if err != nil { t.Fatal(err) }
	defer proxy.close()

	req, err := http.NewRequest(http.MethodGet, deploymentProxyTestURL(proxy), nil)
	if err != nil { t.Fatal(err) }
	req.Host = "app.example.test"
	req.Header.Set("X-Forwarded-Proto", "https")
	resp, err := http.DefaultClient.Do(req)
	if err != nil { t.Fatal(err) }
	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil { t.Fatal(readErr) }
	if got, want := string(body), "app.example.test|app.example.test|https"; got != want {
		t.Fatalf("proxy did not preserve public request identity: got %q want %q", got, want)
	}
}

func TestDeploymentProxyBindsAllIPv4Interfaces(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	defer backend.Close()

	proxy, err := newDeploymentProxy(reserveTestPort(t), backend.Listener.Addr().(*net.TCPAddr).Port)
	if err != nil { t.Fatal(err) }
	defer proxy.close()

	address, ok := proxy.listener.Addr().(*net.TCPAddr)
	if !ok { t.Fatalf("unexpected listener address type: %T", proxy.listener.Addr()) }
	if !address.IP.IsUnspecified() || address.IP.To4() == nil {
		t.Fatalf("public listener must bind all IPv4 interfaces, got %s", address)
	}
	response, err := http.Get(deploymentProxyTestURL(proxy))
	if err != nil { t.Fatalf("request public listener through loopback: %v", err) }
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK { t.Fatalf("unexpected HTTP status: %d", response.StatusCode) }
}
