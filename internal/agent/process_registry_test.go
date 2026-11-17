package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestProcessRegistryPersistsRecords(t *testing.T) {
	root := t.TempDir()
	registry, err := NewProcessRegistry(root)
	if err != nil { t.Fatal(err) }
	want := ProcessRecord{
		SiteID: "demo", PID: 1234, ProcessGroup: 1234, Port: 3000,
		Runtime: "node", WorkingDir: "/tmp/demo", LogPath: "/tmp/demo.log",
		StartedAt: time.Unix(100, 0).UTC(),
	}
	if err := registry.Save(want); err != nil { t.Fatal(err) }

	reloaded, err := NewProcessRegistry(root)
	if err != nil { t.Fatal(err) }
	got, ok := reloaded.Get("demo")
	if !ok { t.Fatal("record missing after reload") }
	if !reflect.DeepEqual(got, want) { t.Fatalf("record mismatch: got %#v, want %#v", got, want) }

	if err := reloaded.Delete("demo"); err != nil { t.Fatal(err) }
	reloaded, err = NewProcessRegistry(root)
	if err != nil { t.Fatal(err) }
	if _, ok := reloaded.Get("demo"); ok { t.Fatal("record still exists after delete") }
}


func TestProcessRegistryScrubsLegacyEnvironmentSecrets(t *testing.T) {
	root := t.TempDir()
	legacy := `{"demo":{"site_id":"demo","pid":1234,"process_group":1234,"port":3000,"runtime":"php","working_dir":"/tmp/demo","log_path":"/tmp/demo.log","started_at":"1970-01-01T00:01:40Z","start_token":"123","extra_env":{"DB_PASSWORD":"do-not-persist-me"}}}`
	path := filepath.Join(root, "processes.json")
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewProcessRegistry(root); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "do-not-persist-me") || strings.Contains(string(got), "extra_env") {
		t.Fatalf("legacy environment secrets were not scrubbed: %s", got)
	}
}


func TestWaitForProcessExitReapsAndRemovesOnlyMatchingRecord(t *testing.T) {
	root := t.TempDir()
	registry, err := NewProcessRegistry(root)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-c", "sleep 0.2")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	token, err := processStartToken(pid)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal(err)
	}
	record := ProcessRecord{SiteID: "demo", PID: pid, ProcessGroup: pid, Port: 3000, Runtime: "php", WorkingDir: root, LogPath: filepath.Join(root, "demo.log"), StartedAt: time.Now(), StartToken: token}
	if err := registry.Save(record); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal(err)
	}
	manager := &ProcessManager{registry: registry, procs: map[string]*Process{"demo": {SiteID: "demo", PID: pid}}}
	manager.waitForProcessExit("demo", pid, token, cmd)
	if _, ok := registry.Get("demo"); ok {
		t.Fatal("exited process record was not removed")
	}
	if _, ok := manager.Get("demo"); ok {
		t.Fatal("exited process remained in memory")
	}
}

func TestProcessRegistryInstancesUseIndependentAtomicTempFiles(t *testing.T) {
	root := t.TempDir()
	first, err := NewProcessRegistry(root)
	if err != nil { t.Fatal(err) }
	second, err := NewProcessRegistry(root)
	if err != nil { t.Fatal(err) }
	start := make(chan struct{})
	errs := make(chan error, 2)
	write := func(registry *ProcessRegistry, site string) {
		<-start
		for i := 0; i < 40; i++ {
			record := ProcessRecord{SiteID:site, PID:1000+i, ProcessGroup:1000+i, Port:3000, Runtime:"nodejs", WorkingDir:root, LogPath:filepath.Join(root, site+".log"), StartedAt:time.Now(), StartToken:"test"}
			if err := registry.Save(record); err != nil { errs <- err; return }
		}
		errs <- nil
	}
	go write(first, "first")
	go write(second, "second")
	close(start)
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil { t.Fatalf("concurrent registry write failed: %v", err) }
	}
	if _, err := os.Stat(filepath.Join(root, "processes.json")); err != nil { t.Fatalf("registry file missing: %v", err) }
}
