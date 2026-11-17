package agent

import (
	"os"
	"os/exec"
	"path/filepath"
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
	if err := manager.persistDeploymentTransaction(tx); err != nil {
		t.Fatal(err)
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
	manager := &ProcessManager{
		DataRoot: root, registry: registry,
		procs: map[string]*Process{"demo": {SiteID: "demo", PID: pid}},
		intentionalStops: make(map[string]bool), restartAttempts: make(map[string]int),
	}
	tx := deploymentTransaction{Version: 1, SiteID: "demo", State: "activating", CandidateCommit: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}
	if err := manager.rollbackDeploymentTransaction(tx); err != nil {
		t.Fatal(err)
	}
	if _, exists := registry.Get("demo"); exists {
		t.Fatal("candidate process record remained after interrupted deployment recovery")
	}
	_ = cmd.Wait()
	if isAlive(pid) {
		t.Fatal("candidate process remained alive after interrupted deployment recovery")
	}
}
