package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
