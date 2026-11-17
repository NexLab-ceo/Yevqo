package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"veltrix/internal/shared"
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

func TestPruneSiteReleasesRejectsSymlinkHistoryRoot(t *testing.T) {
	root := t.TempDir()
	site := filepath.Join(root, "sites", "demo")
	if err := os.MkdirAll(site, 0700); err != nil { t.Fatal(err) }
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(site, ".yevqo")); err != nil { t.Skipf("symlinks unavailable: %v", err) }
	manager := &ProcessManager{DataRoot: root}
	manager.registry, manager.registryErr = NewProcessRegistry(root)
	if _, err := manager.PruneSiteReleases("demo", 1); err == nil {
		t.Fatal("expected redirected release history to be rejected")
	}
}

func TestPruneSiteReleasesCommand(t *testing.T) {
	root := t.TempDir()
	releases := filepath.Join(root, "sites", "demo", ".yevqo", "releases")
	if err := os.MkdirAll(releases, 0700); err != nil { t.Fatal(err) }
	for _, commit := range []string{strings.Repeat("e", 40), strings.Repeat("f", 40)} {
		if err := os.MkdirAll(filepath.Join(releases, commit), 0700); err != nil { t.Fatal(err) }
	}
	manager := &ProcessManager{DataRoot: root}
	manager.registry, manager.registryErr = NewProcessRegistry(root)
	handler := &CommandHandler{processes: manager}
	payload, err := shared.Marshal(shared.PruneSiteReleases{SiteId: "demo", Keep: 1})
	if err != nil { t.Fatal(err) }
	result := handler.HandleCommand(&shared.Command{Id: "prune-1", Action: "prune_site_releases", Payload: payload})
	if !result.Ok { t.Fatalf("prune command failed: %s", result.Message) }
	var response shared.PruneSiteReleasesResult
	if err := shared.Unmarshal(result.Payload, &response); err != nil { t.Fatal(err) }
	if len(response.Removed) != 1 { t.Fatalf("expected one removed release, got %#v", response.Removed) }
}

func TestGitPullSiteUsesLifecycleLock(t *testing.T) {
	manager := &ProcessManager{DataRoot: t.TempDir()}
	manager.operationMu.Lock()
	done := make(chan error, 1)
	go func() {
		_, err := manager.GitPullSite("../invalid")
		done <- err
	}()
	select {
	case err := <-done:
		manager.operationMu.Unlock()
		t.Fatalf("Git pull bypassed lifecycle lock: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	manager.operationMu.Unlock()
	select {
	case err := <-done:
		if err == nil { t.Fatal("expected invalid site id error") }
	case <-time.After(time.Second):
		t.Fatal("Git pull remained blocked after lifecycle lock was released")
	}
}
