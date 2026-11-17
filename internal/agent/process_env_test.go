package agent

import "testing"

func TestValidManagedEnvironmentKey(t *testing.T) {
	valid := []string{"APP_ENV", "API_KEY", "SENTRY_DSN", "FEATURE_FLAG_2", "DB_CONNECTION", "DB_PASSWORD", "VELTRIX_DATABASES"}
	for _, key := range valid {
		if !validManagedEnvironmentKey(key) { t.Errorf("expected %q to be accepted", key) }
	}
	invalid := []string{"", "app_env", "2BAD", "BAD-NAME", "PATH", "HOME", "PREFIX", "PORT", "HOST", "LD_PRELOAD", "DB_CUSTOM", "VELTRIX_SECRET", "APP;PATH"}
	for _, key := range invalid {
		if validManagedEnvironmentKey(key) { t.Errorf("expected %q to be rejected", key) }
	}
}
