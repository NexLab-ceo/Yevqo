package agent

import (
	"testing"
	"time"
)

func TestRestartDelayIsBoundedExponential(t *testing.T) {
	want := []time.Duration{2*time.Second, 4*time.Second, 8*time.Second, 8*time.Second}
	for attempt, expected := range want {
		if got := restartDelay(attempt); got != expected {
			t.Fatalf("restartDelay(%d) = %s, want %s", attempt, got, expected)
		}
	}
	if maxAutomaticRestarts != 3 { t.Fatalf("maxAutomaticRestarts = %d, want 3", maxAutomaticRestarts) }
}

func TestCloneEnvIsIndependent(t *testing.T) {
	src := map[string]string{"PORT":"8080"}
	dst := cloneEnv(src)
	dst["PORT"] = "9090"
	if src["PORT"] != "8080" { t.Fatal("cloneEnv mutated source map") }
}
