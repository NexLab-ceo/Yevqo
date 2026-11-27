package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigMigratesLegacyStaticPathToExplicitDataRoot(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	config := map[string]any{
		"listen_addr": ":8080",
		"data_root": "/sdcard/veltrix-control",
		"static_dir": "/sdcard/veltrix-control/public",
		"mysql_dsn": "example",
		"admin_username": "admin",
	}
	b, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := loadConfig("", root)
	if cfg.DataRoot != root {
		t.Fatalf("DataRoot = %q, want %q", cfg.DataRoot, root)
	}
	if want := filepath.Join(root, "public"); cfg.StaticDir != want {
		t.Fatalf("StaticDir = %q, want %q", cfg.StaticDir, want)
	}
	if cfg.MySQLDSN != "example" {
		t.Fatalf("MySQLDSN unexpectedly changed: %q", cfg.MySQLDSN)
	}
}

func TestLoadConfigPreservesCustomStaticPath(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	config := map[string]any{
		"data_root": "/old/root",
		"static_dir": "/custom/static",
		"mysql_dsn": "example",
	}
	b, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := loadConfig("", root)
	if cfg.DataRoot != root {
		t.Fatalf("DataRoot = %q, want %q", cfg.DataRoot, root)
	}
	if cfg.StaticDir != "/custom/static" {
		t.Fatalf("custom StaticDir changed: %q", cfg.StaticDir)
	}
}

func TestLoadConfigCreatesPrivateConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "config", "config.json")
	_ = loadConfig(path, "")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat new config: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("new config permissions = %04o, want 0600", got)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat config directory: %v", err)
	}
	if got := dirInfo.Mode().Perm() & 0o077; got != 0 {
		t.Fatalf("config directory permissions = %04o, want private", dirInfo.Mode().Perm())
	}
}

func TestSecureConfigFileTightensExistingPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"mysql_dsn":"secret"}`), 0o644); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
	exists, err := secureConfigFile(path)
	if err != nil || !exists {
		t.Fatalf("secureConfigFile = (%v, %v), want (true, nil)", exists, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("existing config permissions = %04o, want 0600", got)
	}
}

func TestSecureConfigFileRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.json")
	link := filepath.Join(root, "config.json")
	if err := os.WriteFile(target, []byte(`{"mysql_dsn":"secret"}`), 0o600); err != nil {
		t.Fatalf("write target config: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create config symlink: %v", err)
	}
	if _, err := secureConfigFile(link); err == nil {
		t.Fatal("expected symlink config to be rejected")
	}
}
