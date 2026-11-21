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
