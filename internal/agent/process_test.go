package agent

import (
	"os"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"time"
	"path/filepath"
	"strings"
	"testing"

	"veltrix/internal/shared"
)

func TestValidSiteID(t *testing.T) {
	for _, id := range []string{"blog", "my-site_2", "a"} {
		if !validSiteID(id) { t.Fatalf("expected %q to be valid", id) }
	}
	for _, id := range []string{"", "../escape", "a/b", "site name", strings.Repeat("a", 65)} {
		if validSiteID(id) { t.Fatalf("expected %q to be invalid", id) }
	}
}

func TestCreateSiteIsNonDestructive(t *testing.T) {
	root := t.TempDir()
	manager := NewProcessManager(root)
	if err := manager.CreateSite("blog", 8000, "php"); err != nil { t.Fatal(err) }
	index := filepath.Join(root, "sites", "blog", "index.php")
	if _, err := os.Stat(index); err != nil { t.Fatalf("starter file missing: %v", err) }
	if err := os.WriteFile(index, []byte("keep me"), 0o600); err != nil { t.Fatal(err) }
	if err := manager.CreateSite("blog", 8000, "php"); err == nil { t.Fatal("expected existing site to be rejected") }
	got, err := os.ReadFile(index); if err != nil { t.Fatal(err) }
	if string(got) != "keep me" { t.Fatalf("existing site data was changed: %q", got) }
}

func TestSafeSiteDirectoryRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	sites := filepath.Join(root, "sites")
	if err := os.MkdirAll(sites, 0o700); err != nil { t.Fatal(err) }
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(sites, "escape")); err != nil { t.Skipf("symlinks unavailable: %v", err) }
	if _, err := safeSiteDirectory(root, "escape"); err == nil { t.Fatal("expected symlink escape to be rejected") }
}


func TestValidDeployBranch(t *testing.T) {
	for _, branch := range []string{"main", "release/2026.11", "feature/new-site", "v1_2"} {
		if !validDeployBranch(branch) { t.Fatalf("expected branch %q to be valid", branch) }
	}
	for _, branch := range []string{"", "-option", "../main", "main..evil", "main//test", "main/", "main.", "branch name", "main\\test", "main@{1}"} {
		if validDeployBranch(branch) { t.Fatalf("expected branch %q to be rejected", branch) }
	}
}

func TestValidDeployRepository(t *testing.T) {
	for _, raw := range []string{"https://github.com/example/site.git", "ssh://git@github.com/example/site.git"} {
		if !validDeployRepository(raw) { t.Fatalf("expected repository URL %q to be valid", raw) }
	}
	for _, raw := range []string{"file:///tmp/site", "https://user:password@example.com/site.git", "https://example.com/site.git\n--upload-pack=evil", "git://example.com/site.git"} {
		if validDeployRepository(raw) { t.Fatalf("expected repository URL %q to be rejected", raw) }
	}
}

func TestTruncateDeployOutput(t *testing.T) {
	input := strings.Repeat("x", (64<<10)+10)
	got := truncateDeployOutput(input)
	if len(got) > (64<<10)+len("\n[output truncated]") { t.Fatalf("output was not bounded: %d bytes", len(got)) }
	if !strings.Contains(got, "[output truncated]") { t.Fatal("truncation marker missing") }
}


func TestDeployRequiresExplicitProjectCodeConsent(t *testing.T) {
	manager := &ProcessManager{}
	req := shared.SiteAction{SiteId: "demo", GitURL: "https://github.com/example/demo.git", Runtime: "static"}
	_, _, _, _, err := manager.DeployGitSite(req, false)
	if err == nil || !strings.Contains(err.Error(), "explicit allow_project_code=true consent") {
		t.Fatalf("expected explicit deployment consent error, got %v", err)
	}
}

func TestNodeStartCommandUsesPackageManagerWithoutShell(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte("{\"scripts\":{\"start\":\"node server.js\"}}"), 0o600); err != nil { t.Fatal(err) }
	cmd, err := nodeStartCommand(root)
	if err != nil { t.Skipf("npm not installed in test environment: %v", err) }
	if cmd.Path == "" || len(cmd.Args) != 3 || cmd.Args[1] != "run" || cmd.Args[2] != "start" { t.Fatalf("unexpected Node start command: %#v", cmd.Args) }
	if strings.Contains(strings.ToLower(cmd.Path), "sh") { t.Fatalf("Node runtime must not invoke a shell: %s", cmd.Path) }
}

func TestCreateNodeSiteWritesStarterFiles(t *testing.T) {
	root := t.TempDir()
	manager := NewProcessManager(root)
	if err := manager.CreateSite("node-demo", 3000, "nodejs"); err != nil { t.Fatal(err) }
	for _, name := range []string{"package.json", "server.js"} {
		if _, err := os.Stat(filepath.Join(root, "sites", "node-demo", name)); err != nil { t.Fatalf("expected Node.js starter file %s: %v", name, err) }
	}
	if err := manager.CreateSite("invalid-node", 3000, "node"); err == nil { t.Fatal("expected unknown runtime to be rejected") }
}

func TestNodeRuntimeStartsServesAndStops(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil { t.Skip("Node.js is not installed") }
	if _, err := exec.LookPath("npm"); err != nil { t.Skip("npm is not installed") }
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil { t.Fatal(err) }
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	root := t.TempDir()
	manager := NewProcessManager(root)
	if err := manager.CreateSite("node-http", port, "nodejs"); err != nil { t.Fatal(err) }
	pid, err := manager.StartSite("node-http", filepath.Join("sites", "node-http"), port, "nodejs")
	if err != nil { t.Fatalf("start Node.js runtime: %v", err) }
	if pid <= 0 { t.Fatalf("invalid Node.js PID: %d", pid) }
	defer func() { _ = manager.StopSite("node-http") }()
	response, err := http.Get("http://127.0.0.1:" + strconv.Itoa(port))
	if err != nil { t.Fatalf("request Node.js site: %v", err) }
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK { t.Fatalf("unexpected HTTP status: %d", response.StatusCode) }
	if err := manager.StopSite("node-http"); err != nil { t.Fatalf("stop Node.js runtime: %v", err) }
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(port), 100*time.Millisecond)
		if err != nil { return }
		_ = conn.Close()
		time.Sleep(25*time.Millisecond)
	}
	t.Fatal("Node.js listener remained open after StopSite")
}
