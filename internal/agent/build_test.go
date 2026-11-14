package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func project(t *testing.T, files ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { t.Fatal(err) }
		if err := os.WriteFile(path, []byte(""), 0o644); err != nil { t.Fatal(err) }
	}
	return root
}

func TestAnalyzeProject(t *testing.T) {
	cases := []struct{ name string; files []string; provider, stack string; needsConfig bool }{
		{"Docker wins", []string{"Dockerfile","package.json"}, "dockerfile","container",false},
		{"pnpm", []string{"package.json","pnpm-lock.yaml"}, "auto","nodejs",false},
		{"Python", []string{"pyproject.toml"}, "auto","python",false},
		{"Rust", []string{"Cargo.toml"}, "auto","rust",false},
		{"Go", []string{"go.mod"}, "auto","go",false},
		{"Static", []string{"index.html"}, "static","static",false},
		{"Unknown", []string{"README.md","src/main.xyz"}, "auto","unknown",true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := AnalyzeProject(project(t, tc.files...))
			if err != nil { t.Fatal(err) }
			if got.Provider != tc.provider || got.Stack != tc.stack || got.NeedsConfig != tc.needsConfig {
				t.Fatalf("unexpected plan: %+v", got)
			}
		})
	}
}

func TestAnalyzeProjectRejectsFileRoot(t *testing.T) {
	root := project(t, "README.md")
	if err := os.WriteFile(filepath.Join(root,"not-dir"), []byte("x"), 0o644); err != nil { t.Fatal(err) }
	if _, err := AnalyzeProject(filepath.Join(root,"not-dir")); err == nil { t.Fatal("expected non-directory root to fail") }
}
