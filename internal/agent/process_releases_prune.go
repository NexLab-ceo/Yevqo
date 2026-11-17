package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// PruneSiteReleases removes old immutable releases while always preserving the
// active release. It is deliberately explicit: deployments never delete
// rollback targets automatically.
func (m *ProcessManager) PruneSiteReleases(siteID string, keep int) ([]string, error) {
	m.deployMu.Lock()
	defer m.deployMu.Unlock()

	if !validSiteID(siteID) {
		return nil, fmt.Errorf("invalid site id")
	}
	if keep < 1 || keep > 50 {
		return nil, fmt.Errorf("keep must be between 1 and 50")
	}
	if m.registry == nil || m.registryErr != nil {
		return nil, fmt.Errorf("process registry unavailable: %v", m.registryErr)
	}

	siteRoot, err := safeSiteDirectory(m.DataRoot, siteID)
	if err != nil {
		return nil, err
	}
	metadataRoot := filepath.Join(siteRoot, ".yevqo")
	metadataInfo, err := os.Lstat(metadataRoot)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect deployment metadata: %w", err)
	}
	if !metadataInfo.IsDir() || metadataInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("deployment metadata must be a real directory")
	}
	releasesRoot := filepath.Join(metadataRoot, "releases")
	rootInfo, err := os.Lstat(releasesRoot)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect release history: %w", err)
	}
	if !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("release history must be a real directory, not a symlink")
	}
	resolvedSiteRoot, err := filepath.EvalSymlinks(siteRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve site directory: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(releasesRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve release history: %w", err)
	}
	rootRel, err := filepath.Rel(resolvedSiteRoot, resolvedRoot)
	if err != nil || rootRel != filepath.Join(".yevqo", "releases") {
		return nil, fmt.Errorf("release history resolves outside the site directory")
	}

	activeCommit := ""
	if record, ok := m.registry.Get(siteID); ok {
		prefix := filepath.Join("sites", siteID, ".yevqo", "releases") + string(filepath.Separator)
		runPath := filepath.Clean(record.RunPath)
		if strings.HasPrefix(runPath, prefix) {
			rest := strings.TrimPrefix(runPath, prefix)
			candidate := strings.SplitN(rest, string(filepath.Separator), 2)[0]
			if validReleaseCommit(candidate) {
				activeCommit = candidate
			}
		}
	}

	entries, err := os.ReadDir(resolvedRoot)
	if err != nil {
		return nil, fmt.Errorf("read release history: %w", err)
	}
	type releaseEntry struct {
		commit string
		date   string
	}
	releases := make([]releaseEntry, 0, len(entries))
	for _, entry := range entries {
		if !validReleaseCommit(entry.Name()) {
			continue
		}
		info, err := os.Lstat(filepath.Join(resolvedRoot, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("inspect release %s: %w", entry.Name(), err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("refusing to prune non-directory or symlink release %s", entry.Name())
		}
		date := info.ModTime().UTC().Format("2006-01-02T15:04:05Z")
		if output, gitErr := execGitCommitDate(filepath.Join(resolvedRoot, entry.Name())); gitErr == nil && output != "" {
			date = output
		}
		releases = append(releases, releaseEntry{commit: entry.Name(), date: date})
	}
	sort.Slice(releases, func(i, j int) bool {
		if releases[i].date == releases[j].date {
			return releases[i].commit > releases[j].commit
		}
		return releases[i].date > releases[j].date
	})

	keepSet := make(map[string]bool, keep+1)
	for i := 0; i < len(releases) && i < keep; i++ {
		keepSet[releases[i].commit] = true
	}
	if activeCommit != "" {
		keepSet[activeCommit] = true
	}

	removed := make([]string, 0)
	for _, release := range releases {
		if keepSet[release.commit] {
			continue
		}
		path := filepath.Join(resolvedRoot, release.commit)
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return removed, fmt.Errorf("revalidate release %s: %w", release.commit, err)
		}
		rel, err := filepath.Rel(resolvedRoot, resolved)
		if err != nil || rel != release.commit {
			return removed, fmt.Errorf("refusing to remove release outside release history: %s", release.commit)
		}
		if err := os.RemoveAll(path); err != nil {
			return removed, fmt.Errorf("remove release %s: %w", release.commit, err)
		}
		removed = append(removed, release.commit)
	}
	return removed, nil
}

func execGitCommitDate(path string) (string, error) {
	out, err := exec.Command("git", "-C", path, "show", "-s", "--format=%cI", "HEAD").Output()
	return strings.TrimSpace(string(out)), err
}
