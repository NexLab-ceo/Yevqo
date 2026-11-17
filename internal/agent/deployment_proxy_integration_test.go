package agent

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeProxyTestRelease(t *testing.T, root, siteID, commit, body string) string {
	t.Helper()
	release := filepath.Join(root, "sites", siteID, ".yevqo", "releases", commit)
	if err := os.MkdirAll(release, 0o700); err != nil { t.Fatal(err) }
	script := `from http.server import BaseHTTPRequestHandler, HTTPServer
import os
class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        body = open("version.txt", "rb").read()
        self.send_response(200)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)
HTTPServer(("127.0.0.1", int(os.environ["PORT"])), Handler).serve_forever()
`
	if err := os.WriteFile(filepath.Join(release, "serve.py"), []byte(script), 0o600); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(release, "version.txt"), []byte(body), 0o600); err != nil { t.Fatal(err) }
	return filepath.Join("sites", siteID, ".yevqo", "releases", commit)
}

func TestBlueGreenDeploymentKeepsStablePortAndSwitchesRelease(t *testing.T) {
	if _, err := execLookPathPython(); err != nil { t.Skip("python3 is required for blue-green integration test") }
	root := t.TempDir()
	siteID := "bluegreen"
	if err := os.MkdirAll(filepath.Join(root, "sites", siteID), 0o700); err != nil { t.Fatal(err) }
	registry, err := NewProcessRegistry(root)
	if err != nil { t.Fatal(err) }
	manager := &ProcessManager{DataRoot:root, registry:registry, procs:make(map[string]*Process),
		proxies:make(map[string]*deploymentProxy), restartAttempts:make(map[string]int), intentionalStops:make(map[string]bool)}
	publicListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil { t.Fatal(err) }
	publicPort := publicListener.Addr().(*net.TCPAddr).Port
	_ = publicListener.Close()

	readBody := func(want string) {
		t.Helper()
		deadline := time.Now().Add(5*time.Second)
		for time.Now().Before(deadline) {
			resp, requestErr := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", publicPort))
			if requestErr == nil {
				body, readErr := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if readErr == nil && resp.StatusCode == http.StatusOK && strings.TrimSpace(string(body)) == want { return }
			}
			time.Sleep(30*time.Millisecond)
		}
		t.Fatalf("stable listener did not serve release body %q", want)
	}

	firstCommit := strings.Repeat("a", 40)
	firstPath := writeProxyTestRelease(t, root, siteID, firstCommit, "release-one")
	firstTx := deploymentTransaction{Version:1, SiteID:siteID, State:"activating", CandidateCommit:firstCommit}
	firstPID, err := manager.activateBlueGreenRelease(context.Background(), firstTx, firstPath, publicPort, "static", nil, "python3 serve.py", "/")
	if err != nil { t.Fatalf("activate first release: %v", err) }
	firstRecord, ok := registry.Get(siteID)
	if !ok || firstRecord.PID != firstPID || firstRecord.BackendPort == 0 || firstRecord.Port != publicPort {
		t.Fatalf("first release has invalid stable/backend port record: %#v", firstRecord)
	}
	readBody("release-one")

	secondCommit := strings.Repeat("b", 40)
	secondPath := writeProxyTestRelease(t, root, siteID, secondCommit, "release-two")
	secondTx := deploymentTransaction{Version:1, SiteID:siteID, State:"activating", CandidateCommit:secondCommit, HadPrevious:true, Previous:firstRecord}
	secondPID, err := manager.activateBlueGreenRelease(context.Background(), secondTx, secondPath, publicPort, "static", nil, "python3 serve.py", "/")
	if err != nil { t.Fatalf("activate second release: %v", err) }
	secondRecord, ok := registry.Get(siteID)
	if !ok || secondRecord.PID != secondPID || secondRecord.Port != publicPort || secondRecord.BackendPort == firstRecord.BackendPort {
		t.Fatalf("second release did not switch backend: %#v", secondRecord)
	}
	readBody("release-two")
	if verifyProcessRecord(firstRecord) { t.Fatal("previous release process was not stopped after traffic switch") }
	if err := manager.StopSite(siteID); err != nil { t.Fatalf("stop blue-green site: %v", err) }
}

func execLookPathPython() (string, error) {
	return exec.LookPath("python3")
}
