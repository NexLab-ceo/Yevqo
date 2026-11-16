package agent

import (
	"strings"
	"testing"
)

func TestValidDeployBranch(t *testing.T) {
	for _, branch := range []string{"main", "release/v1.2", "feature_fix", "26.3"} {
		if !validDeployBranch(branch) { t.Errorf("expected valid branch %q", branch) }
	}
	for _, branch := range []string{"", "-main", "../main", "main..evil", "main/", "main\\evil", "main;rm", "main\n"} {
		if validDeployBranch(branch) { t.Errorf("expected invalid branch %q", branch) }
	}
}

func TestValidDeployRepository(t *testing.T) {
	for _, repo := range []string{"https://github.com/example/project.git", "ssh://git@github.com/example/project.git"} {
		if !validDeployRepository(repo) { t.Errorf("expected valid repository %q", repo) }
	}
	for _, repo := range []string{"", "file:///etc/passwd", "https://user:password@example.com/repo", "https://example.com/repo\n--upload-pack=evil", "git://example.com/repo"} {
		if validDeployRepository(repo) { t.Errorf("expected invalid repository %q", repo) }
	}
}

func TestTruncateDeployOutput(t *testing.T) {
	short := "build complete"
	if got := truncateDeployOutput(short); got != short { t.Fatalf("short output changed: %q", got) }
	large := strings.Repeat("x", (64<<10)+20)
	got := truncateDeployOutput(large)
	if len(got) <= 64<<10 || !strings.HasSuffix(got, "[output truncated]") {
		t.Fatalf("large output wasn't truncated with marker (len=%d)", len(got))
	}
}
