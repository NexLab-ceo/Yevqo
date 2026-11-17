package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPruneSiteReleasesPreservesNewestAndActive(t *testing.T) {
	root := t.TempDir()
	releases := filepath.Join(root, "sites", "demo", ".yevqo", "releases")
	if err := os.MkdirAll(releases, 0700); err != nil { t.Fatal(err) }
	commits := []string{strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)}
	for i, commit := range commits {
		dir := filepath.Join(releases, commit)
		if err := os.MkdirAll(dir, 0700); err != nil { t.Fatal(err) }
		stamp := time.Unix(int64(i+1), 0)
		if err := os.Chtimes(dir, stamp, stamp); err != nil { t.Fatal(err) }
	}
	manager := &ProcessManager{DataRoot: root}
	manager.registry, manager.registryErr = NewProcessRegistry(root)
	active := filepath.Join("sites", "demo", ".yevqo", "releases", commits[0])
	if err := manager.registry.Save(ProcessRecord{SiteID: "demo", PID: 1, ProcessGroup: 1, RunPath: active}); err != nil { t.Fatal(err) }
	removed, err := manager.PruneSiteReleases("demo", 1)
	if err != nil { t.Fatal(err) }
	if len(removed) != 1 || removed[0] != commits[1] { t.Fatalf("unexpected removed releases: %#v", removed) }
	for _, commit := range []string{commits[0], commits[2]} {
		if _, err := os.Stat(filepath.Join(releases, commit)); err != nil { t.Fatalf("release %s should be retained: %v", commit, err) }
	}
	if _, err := os.Stat(filepath.Join(releases, commits[1])); !os.IsNotExist(err) { t.Fatalf("old release should be removed, stat err=%v", err) }
}

func TestPruneSiteReleasesRejectsSymlinkRelease(t *testing.T) {
	root := t.TempDir()
	releases := filepath.Join(root, "sites", "demo", ".yevqo", "releases")
	if err := os.MkdirAll(releases, 0700); err != nil { t.Fatal(err) }
	outside := t.TempDir()
	commit := strings.Repeat("d", 40)
	if err := os.Symlink(outside, filepath.Join(releases, commit)); err != nil { t.Skipf("symlinks unavailable: %v", err) }
	manager := &ProcessManager{DataRoot: root}
	manager.registry, manager.registryErr = NewProcessRegistry(root)
	if _, err := manager.PruneSiteReleases("demo", 1); err == nil { t.Fatal("expected symlink release to be rejected") }
}

func TestPruneSiteReleasesValidatesKeepCount(t *testing.T) {
	manager := &ProcessManager{DataRoot: t.TempDir()}
	manager.registry, manager.registryErr = NewProcessRegistry(manager.DataRoot)
	for _, keep := range []int{0, 51} {
		if _, err := manager.PruneSiteReleases("demo", keep); err == nil { t.Fatalf("expected keep=%d to be rejected", keep) }
	}
}
