package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExecutableAvailableUsesPath(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-tool")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", dir)
	if !executableAvailable("fake-tool") {
		t.Fatal("expected executable to be found in PATH")
	}
	t.Setenv("PATH", "")
	if executableAvailable("fake-tool") {
		t.Fatal("did not expect executable to be found with empty PATH")
	}
	t.Setenv("PATH", oldPath)
}

func TestBuildCapabilitiesNeverExecutesCustomCommands(t *testing.T) {
	t.Setenv("PATH", "")
	got := DetectBuildCapabilities()
	if got.CustomCommandsIsolated {
		t.Fatal("custom commands must remain disabled without an isolation backend")
	}
	if got.CanBuildDockerfile || got.CanBuildNixpacks {
		t.Fatal("container providers must be unavailable without a backend")
	}
	if len(got.Warnings) == 0 {
		t.Fatal("expected actionable capability warnings")
	}
}
