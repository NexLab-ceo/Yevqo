package control

import "testing"

func TestParseGitHubRepositoryName(t *testing.T) {
	tests := []struct {
		input string
		owner string
		name string
		full string
		wantErr bool
	}{
		{input: "NexLab-ceo/Yevqo", owner: "NexLab-ceo", name: "Yevqo", full: "NexLab-ceo/Yevqo"},
		{input: "https://github.com/NexLab-ceo/Yevqo.git", owner: "NexLab-ceo", name: "Yevqo", full: "NexLab-ceo/Yevqo"},
		{input: "owner/repo-name_2", owner: "owner", name: "repo-name_2", full: "owner/repo-name_2"},
		{input: "owner", wantErr: true},
		{input: "owner/repo/extra", wantErr: true},
		{input: "../repo", wantErr: true},
		{input: "owner/repo?redirect=1", wantErr: true},
		{input: "https://evil.example/owner/repo", wantErr: true},
		{input: "owner/repo;rm", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			owner, name, full, err := parseGitHubRepositoryName(tt.input)
			if tt.wantErr {
				if err == nil { t.Fatal("expected invalid repository name to be rejected") }
				return
			}
			if err != nil { t.Fatalf("unexpected error: %v", err) }
			if owner != tt.owner || name != tt.name || full != tt.full {
				t.Fatalf("got %q/%q (%q), want %q/%q (%q)", owner, name, full, tt.owner, tt.name, tt.full)
			}
		})
	}
}

func TestGitHubRepositoryPartRejectsUnsafeSegments(t *testing.T) {
	for _, value := range []string{"", ".", "..", "a/b", "a\\b", "repo?x", "repo#x", "repo name"} {
		if githubRepositoryPart.MatchString(value) {
			t.Errorf("unsafe repository segment accepted: %q", value)
		}
	}
}
