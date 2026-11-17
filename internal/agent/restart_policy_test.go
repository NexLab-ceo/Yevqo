package agent

import (
	"errors"
	"testing"
	"time"
)

func TestRestartDelayIsBoundedExponential(t *testing.T) {
	want := []time.Duration{2*time.Second, 2*time.Second, 4*time.Second, 8*time.Second}
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


func TestShouldRestartHonorsPolicy(t *testing.T) {
 failure := errors.New("exit status 1")
 cases := []struct { policy string; err error; want bool }{
  {"never", failure, false},
  {"never", nil, false},
  {"on-failure", failure, true},
  {"on-failure", nil, false},
  {"always", failure, true},
  {"always", nil, true},
 }
 for _, test := range cases {
  if got := shouldRestart(test.policy, test.err); got != test.want {
   t.Errorf("shouldRestart(%q, %v) = %v, want %v", test.policy, test.err, got, test.want)
  }
 }
}

func TestRestartPolicyPersistsAcrossProcessState(t *testing.T) {
 root := t.TempDir()
 manager := &ProcessManager{DataRoot: root}
 if err := manager.SetRestartPolicy("demo", "always"); err != nil { t.Fatal(err) }
 reloaded := &ProcessManager{DataRoot: root}
 got, err := reloaded.GetRestartPolicy("demo")
 if err != nil { t.Fatal(err) }
 if got != "always" { t.Fatalf("policy = %q, want always", got) }
 if got, err := reloaded.GetRestartPolicy("new-site"); err != nil || got != "on-failure" {
  t.Fatalf("default policy = %q, err=%v; want on-failure", got, err)
 }
 if err := reloaded.SetRestartPolicy("../escape", "always"); err == nil { t.Fatal("expected invalid site id to be rejected") }
 if err := reloaded.SetRestartPolicy("demo", "sometimes"); err == nil { t.Fatal("expected invalid policy to be rejected") }
}
