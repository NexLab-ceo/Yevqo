package control

import ("testing"; "strings")

func TestSiteEnvironmentKeyValidation(t *testing.T) {
	for _, key := range []string{"APP_ENV", "NEXT_PUBLIC_API_URL", "FEATURE_FLAG_1", "_PRIVATE"} {
		if !validSiteEnvKey(key) {
			t.Errorf("valid environment key %q was rejected", key)
		}
	}
	for _, key := range []string{
		"", "lowercase", "1STARTS_WITH_NUMBER", "HAS-DASH", "HAS.DOT",
		"PATH", "HOME", "PREFIX", "TMPDIR", "PORT", "HOST",
		"DB_PASSWORD", "DB_CONNECTION", "VELTRIX_SECRET", "LD_PRELOAD",
		"TOO_LONG_" + string(make([]byte, 70)),
	} {
		if validSiteEnvKey(key) {
			t.Errorf("invalid or reserved environment key %q was accepted", key)
		}
	}
}

func TestGitBranchValidation(t *testing.T) {
	for _, branch := range []string{"main", "release/1.2", "feature/add-login", "v2.0.0"} {
		if !gitBranchPattern.MatchString(branch) || containsInvalidGitBranchSuffix(branch) {
			t.Errorf("valid branch %q was rejected", branch)
		}
	}
	for _, branch := range []string{"", "../main", "main..evil", "main@", "main/", "main.", "feature/@{upstream}", "has space"} {
		if gitBranchPattern.MatchString(branch) && !containsInvalidGitBranchSuffix(branch) {
			t.Errorf("invalid branch %q passed validation", branch)
		}
	}
}

func containsInvalidGitBranchSuffix(branch string) bool {
	return branch == "" || strings.Contains(branch, "..") || strings.Contains(branch, "@{") || strings.HasSuffix(branch, "/") || strings.HasSuffix(branch, ".")
}

func TestGitRepositoryAndRootDirectoryValidation(t *testing.T) {
	for _, raw := range []string{"https://github.com/acme/app.git", "ssh://github.com/acme/app.git"} {
		if !validateGitRepository(raw) {
			t.Errorf("valid Git URL %q was rejected", raw)
		}
	}
	for _, raw := range []string{"", "file:///etc/passwd", "https://user:pass@example.com/repo", "https://example.com/repo\n"} {
		if validateGitRepository(raw) {
			t.Errorf("unsafe Git URL %q was accepted", raw)
		}
	}
	for _, raw := range []string{"", ".", "apps/web", "packages/api"} {
		if _, ok := validateSiteRootDir(raw); !ok {
			t.Errorf("valid site root %q was rejected", raw)
		}
	}
	for _, raw := range []string{"..", "../outside", "apps/../../outside", "/etc", "apps\\web"} {
		if _, ok := validateSiteRootDir(raw); ok {
			t.Errorf("site root traversal %q was accepted", raw)
		}
	}
}
