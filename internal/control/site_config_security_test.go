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

func TestSiteDatabaseEnvironmentFilterRequiresPairedScope(t *testing.T) {
	for _, tc := range []struct { projectID, environmentID int64; wantError bool }{
		{0, 0, false}, {7, 0, true}, {0, 3, true}, {7, 3, false},
	} {
		filter, args, err := siteDatabaseEnvironmentFilter(tc.projectID, tc.environmentID)
		if (err != nil) != tc.wantError { t.Fatalf("scope (%d,%d) error=%v, wantError=%v", tc.projectID, tc.environmentID, err, tc.wantError) }
		if tc.wantError { continue }
		if tc.projectID == 0 {
			if filter != "" || len(args) != 0 { t.Fatalf("unscoped query unexpectedly filtered: %q %#v", filter, args) }
			continue
		}
		if !strings.Contains(filter, "project_environment_databases") || len(args) != 2 || args[0] != int64(7) || args[1] != int64(3) {
			t.Fatalf("environment filter is not scoped: %q %#v", filter, args)
		}
	}
}
