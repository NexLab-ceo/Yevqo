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
