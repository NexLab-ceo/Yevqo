package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigUsesYevqoDataRootAndPrivateFileMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "config", "agent.json")
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load new config: %v", err)
	}
	if cfg.DataRoot != DefaultDataRoot {
		t.Fatalf("new config data root = %q, want %q", cfg.DataRoot, DefaultDataRoot)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config permissions = %04o, want 0600", got)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat config directory: %v", err)
	}
	if got := dirInfo.Mode().Perm() & 0o077; got != 0 {
		t.Fatalf("config directory is accessible to group/other: mode %04o", dirInfo.Mode().Perm())
	}
}

func TestLoadConfigTightensExistingFilePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.json")
	if err := os.WriteFile(path, []byte(`{"server_url":"wss://example.invalid/ws","token":"secret","device_id":"device-1","data_root":"/sdcard/yevqo-agent"}`), 0o644); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
	if _, err := LoadConfig(path); err != nil {
		t.Fatalf("load existing config: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("existing config permissions = %04o, want 0600", got)
	}
}

func TestLoadConfigRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.json")
	link := filepath.Join(root, "agent.json")
	if err := os.WriteFile(target, []byte(`{"device_id":"device-1"}`), 0o600); err != nil {
		t.Fatalf("write target config: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create config symlink: %v", err)
	}
	if _, err := LoadConfig(link); err == nil {
		t.Fatal("expected symlink config to be rejected")
	}
}
