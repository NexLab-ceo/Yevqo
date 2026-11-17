package agent

import (
	"testing"
	"time"
)

func TestConfiguredDeploymentTimeout(t *testing.T) {
	got, err := configuredDeploymentTimeout(0, 10*time.Minute, 30, 1800, "clone")
	if err != nil || got != 10*time.Minute { t.Fatalf("default timeout = %s, %v", got, err) }
	got, err = configuredDeploymentTimeout(90, 10*time.Minute, 30, 1800, "clone")
	if err != nil || got != 90*time.Second { t.Fatalf("configured timeout = %s, %v", got, err) }
	for _, value := range []int{-1, 29, 1801} {
		if _, err := configuredDeploymentTimeout(value, 10*time.Minute, 30, 1800, "clone"); err == nil {
			t.Fatalf("expected %d seconds to be rejected", value)
		}
	}
	if _, err := configuredDeploymentTimeout(300, 15*time.Second, 3, 300, "health check"); err != nil {
		t.Fatalf("maximum allowed timeout rejected: %v", err)
	}
	if _, err := configuredDeploymentTimeout(301, 15*time.Second, 3, 300, "health check"); err == nil {
		t.Fatal("expected timeout above maximum to be rejected")
	}
}
