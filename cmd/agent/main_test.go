package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveAgentConfigPathPrefersPrivatePath(t *testing.T) {
	root := t.TempDir()
	prefix := filepath.Join(root, "prefix")
	private := filepath.Join(prefix, "var", "lib", "veltrix-agent", "config", "agent.json")
	legacy := filepath.Join(root, "legacy", "agent.json")
	if err := os.MkdirAll(filepath.Dir(private), 0o700); err != nil { t.Fatal(err) }
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil { t.Fatal(err) }
	if err := os.WriteFile(private, []byte("{}"), 0o600); err != nil { t.Fatal(err) }
	if err := os.WriteFile(legacy, []byte("{}"), 0o600); err != nil { t.Fatal(err) }
	if got := resolveAgentConfigPath(prefix, "", legacy); got != private {
		t.Fatalf("resolved path = %q, want private path %q", got, private)
	}
}

func TestResolveAgentConfigPathPreservesLegacyConfig(t *testing.T) {
	root := t.TempDir()
	prefix := filepath.Join(root, "prefix")
	legacy := filepath.Join(root, "legacy", "agent.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil { t.Fatal(err) }
	if err := os.WriteFile(legacy, []byte("{}"), 0o600); err != nil { t.Fatal(err) }
	want := legacy
	if got := resolveAgentConfigPath(prefix, "", legacy); got != want {
		t.Fatalf("resolved path = %q, want legacy path %q", got, want)
	}
}

func TestResolveAgentConfigPathUsesPrivatePathForFreshInstall(t *testing.T) {
	prefix := filepath.Join(t.TempDir(), "prefix")
	want := filepath.Join(prefix, "var", "lib", "veltrix-agent", "config", "agent.json")
	if got := resolveAgentConfigPath(prefix, "", filepath.Join(t.TempDir(), "missing", "agent.json")); got != want {
		t.Fatalf("resolved path = %q, want private path %q", got, want)
	}
}

func TestResolveAgentConfigPathHonorsExplicitOverride(t *testing.T) {
	want := filepath.Join(t.TempDir(), "custom", "agent.json")
	if got := resolveAgentConfigPath("/prefix", want, "/legacy/agent.json"); got != want {
		t.Fatalf("resolved path = %q, want override %q", got, want)
	}
}
