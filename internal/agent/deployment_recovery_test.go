package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"net"
	"testing"
	"time"
)

func TestDeploymentTransactionPersistsAndReloads(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sites", "demo"), 0o700); err != nil { t.Fatal(err) }
	manager := NewProcessManager(root)
	tx := deploymentTransaction{
		Version: 1, SiteID: "demo", State: "activating",
		CandidateCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		HadPrevious: true,
		Previous: ProcessRecord{
			SiteID: "demo", PID: 1234, ProcessGroup: 1234, Port: 3000,
			Runtime: "static", WorkingDir: filepath.Join(root, "sites", "demo"),
			LogPath: filepath.Join(root, "logs", "demo.log"), StartToken: "12345",
		},
	}
	if err := manager.createDeploymentTransaction(tx); err != nil {
		t.Fatal(err)
	}
	duplicate := tx
	duplicate.CandidateCommit = "cccccccccccccccccccccccccccccccccccccccc"
	if err := manager.createDeploymentTransaction(duplicate); err == nil {
		t.Fatal("expected unresolved transaction to be protected from overwrite")
	}
	got, exists, err := manager.readDeploymentTransaction("demo")
	if err != nil {
		t.Fatal(err)
	}
	if !exists || got.State != tx.State || got.CandidateCommit != tx.CandidateCommit ||
		!got.HadPrevious || got.Previous.PID != tx.Previous.PID {
		t.Fatalf("unexpected recovered transaction: exists=%v tx=%#v", exists, got)
	}
	if err := manager.removeDeploymentTransaction("demo"); err != nil {
		t.Fatal(err)
	}
	if _, exists, err := manager.readDeploymentTransaction("demo"); err != nil || exists {
		t.Fatalf("transaction should be removed: exists=%v err=%v", exists, err)
	}
}

func TestDeploymentTransactionRejectsInvalidRecord(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sites", "demo"), 0o700); err != nil { t.Fatal(err) }
	manager := NewProcessManager(root)
	path, err := manager.deploymentTransactionPath("demo")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"site_id":"other","state":"activating"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, exists, err := manager.readDeploymentTransaction("demo"); err == nil || exists {
		t.Fatalf("expected invalid transaction to fail closed, exists=%v err=%v", exists, err)
	}
}

func TestInterruptedDeploymentWithoutPreviousStopsCandidate(t *testing.T) {
	root := t.TempDir()
	registry, err := NewProcessRegistry(root)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-c", "sleep 30")
	cmd.SysProcAttr = procAttr()
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start helper process: %v", err)
	}
	pid := cmd.Process.Pid
	token, err := processStartToken(pid)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal(err)
	}
	record := ProcessRecord{
		SiteID: "demo", PID: pid, ProcessGroup: pid, Port: 3000,
		Runtime: "static", WorkingDir: root, LogPath: filepath.Join(root, "demo.log"),
		StartedAt: time.Now().UTC(), StartToken: token,
	}
	if err := registry.Save(record); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sites", "demo"), 0o700); err != nil {
		t.Fatal(err)
	}
	manager := NewProcessManager(root)
	tx := deploymentTransaction{Version: 1, SiteID: "demo", State: "activating", CandidateCommit: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}
	if err := manager.createDeploymentTransaction(tx); err != nil {
		t.Fatal(err)
	}
	// Constructing a fresh manager simulates Agent startup after a hard crash.
	restarted := NewProcessManager(root)
	if _, exists := registry.Get("demo"); exists {
		t.Fatal("candidate process record remained after interrupted deployment recovery")
	}
	if _, exists, err := restarted.readDeploymentTransaction("demo"); err != nil || exists {
		t.Fatalf("recovery journal should be removed after successful rollback: exists=%v err=%v", exists, err)
	}
	_ = cmd.Wait()
	if isAlive(pid) {
		t.Fatal("candidate process remained alive after interrupted deployment recovery")
	}
}


func TestInterruptedDeploymentKeepsStillRunningPreviousProcess(t *testing.T) {
	root := t.TempDir()
	registry, err := NewProcessRegistry(root)
	if err != nil { t.Fatal(err) }
	startHelper := func() (*exec.Cmd, ProcessRecord) {
		cmd := exec.Command("sh", "-c", "sleep 30")
		cmd.SysProcAttr = procAttr()
		if err := cmd.Start(); err != nil { t.Fatalf("start helper: %v", err) }
		token, err := processStartToken(cmd.Process.Pid)
		if err != nil { _ = cmd.Process.Kill(); _ = cmd.Wait(); t.Fatal(err) }
		return cmd, ProcessRecord{SiteID:"demo", PID:cmd.Process.Pid, ProcessGroup:cmd.Process.Pid, Port:3000, Runtime:"static", WorkingDir:root, LogPath:filepath.Join(root,"demo.log"), StartedAt:time.Now().UTC(), StartToken:token}
	}
	previousCmd, previous := startHelper()
	defer func(){ _ = previousCmd.Process.Kill(); _ = previousCmd.Wait() }()
	candidateCmd, candidate := startHelper()
	if err := registry.Save(candidate); err != nil { _ = candidateCmd.Process.Kill(); _ = candidateCmd.Wait(); t.Fatal(err) }
	manager := &ProcessManager{DataRoot:root, registry:registry, procs:map[string]*Process{"demo":{SiteID:"demo",PID:candidate.PID}}, intentionalStops:make(map[string]bool), restartAttempts:make(map[string]int)}
	tx := deploymentTransaction{Version:1, SiteID:"demo", State:"activating", CandidateCommit:"cccccccccccccccccccccccccccccccccccccccc", HadPrevious:true, Previous:previous}
	if err := manager.rollbackDeploymentTransaction(tx); err != nil { t.Fatal(err) }
	got, ok := registry.Get("demo")
	if !ok || got.PID != previous.PID || got.StartToken != previous.StartToken {
		t.Fatalf("previous process identity was not restored: %#v (exists=%v)", got, ok)
	}
	_ = candidateCmd.Wait()
	if isAlive(candidate.PID) { t.Fatal("unconfirmed candidate process remained alive") }
}


func TestInterruptedDeploymentRestartsPreviousReleaseAfterAgentCrash(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil { t.Skip("python3 is not installed") }
	root := t.TempDir()
	siteRoot := filepath.Join(root, "sites", "demo")
	oldRelease := filepath.Join(siteRoot, ".yevqo", "releases", "old")
	if err := os.MkdirAll(oldRelease, 0o700); err != nil { t.Fatal(err) }
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil { t.Fatal(err) }
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	registry, err := NewProcessRegistry(root)
	if err != nil { t.Fatal(err) }
	candidateCmd := exec.Command("sh", "-c", "sleep 30")
	candidateCmd.SysProcAttr = procAttr()
	if err := candidateCmd.Start(); err != nil { t.Skipf("cannot start candidate helper: %v", err) }
	candidatePID := candidateCmd.Process.Pid
	candidateToken, err := processStartToken(candidatePID)
	if err != nil { _ = candidateCmd.Process.Kill(); _ = candidateCmd.Wait(); t.Fatal(err) }
	candidate := ProcessRecord{SiteID:"demo", PID:candidatePID, ProcessGroup:candidatePID, Port:port, Runtime:"static", WorkingDir:siteRoot, LogPath:filepath.Join(root,"demo.log"), StartedAt:time.Now().UTC(), StartToken:candidateToken, RunPath:filepath.Join("sites","demo",".yevqo","releases","candidate")}
	if err := registry.Save(candidate); err != nil { _ = candidateCmd.Process.Kill(); _ = candidateCmd.Wait(); t.Fatal(err) }
	previous := ProcessRecord{
		SiteID:"demo", PID:99999999, ProcessGroup:99999999, Port:port, Runtime:"static",
		WorkingDir:oldRelease, LogPath:filepath.Join(root,"demo.log"), StartedAt:time.Now().Add(-time.Minute),
		StartToken:"stale-process-identity", RunPath:filepath.Join("sites","demo",".yevqo","releases","old"),
		StartCommand:fmt.Sprintf("python3 -m http.server %d --bind 127.0.0.1", port),
	}
	manager := &ProcessManager{DataRoot:root, registry:registry, procs:map[string]*Process{"demo":{SiteID:"demo",PID:candidatePID}}, intentionalStops:make(map[string]bool), restartAttempts:make(map[string]int)}
	tx := deploymentTransaction{Version:1, SiteID:"demo", State:"activating", CandidateCommit:"dddddddddddddddddddddddddddddddddddddddd", HadPrevious:true, Previous:previous}
	if err := manager.rollbackDeploymentTransaction(tx); err != nil { _ = candidateCmd.Process.Kill(); _ = candidateCmd.Wait(); t.Fatal(err) }
	restored, ok := registry.Get("demo")
	if !ok || restored.PID == candidatePID || restored.RunPath != previous.RunPath {
		_ = candidateCmd.Process.Kill(); _ = candidateCmd.Wait()
		t.Fatalf("previous release was not restarted: %#v (exists=%v)", restored, ok)
	}
	if !verifyProcessRecord(restored) || !manager.IsSiteHealthy("demo") {
		_ = manager.StopSite("demo"); _ = candidateCmd.Wait()
		t.Fatal("restored previous release is not healthy")
	}
	if err := manager.StopSite("demo"); err != nil { t.Fatal(err) }
	_ = candidateCmd.Wait()
}
