package agent

import (
 "sync"
 "testing"\n\n "veltrix/internal/shared"
)

func TestDeploymentJobsAreExclusivePerSiteAndCancellable(t *testing.T) {
 manager := &ProcessManager{DataRoot:t.TempDir()}
 var mu sync.Mutex
 var events []string
 manager.SetDeploymentEmitter(func(event shared.DeploymentJobEvent) {
  mu.Lock(); defer mu.Unlock()
  events = append(events, event.State)
 })
 jobID, ctx, err := manager.BeginDeploymentJob("demo")
 if err != nil { t.Fatal(err) }
 if ctx.Err() != nil { t.Fatal("new deployment context is already cancelled") }
 if _, _, err := manager.BeginDeploymentJob("demo"); err == nil { t.Fatal("expected duplicate site job to be rejected") }
 if err := manager.CancelDeploymentJob("demo", "wrong-id"); err == nil { t.Fatal("expected mismatched job ID to be rejected") }
 if err := manager.CancelDeploymentJob("demo", jobID); err != nil { t.Fatal(err) }
 if ctx.Err() == nil { t.Fatal("expected deployment context to be cancelled") }
 manager.FinishDeploymentJob("demo", jobID, "cancelled", "cancelled")
 if _, _, err := manager.BeginDeploymentJob("demo"); err != nil { t.Fatalf("site should accept a new job after completion: %v", err) }
 mu.Lock(); defer mu.Unlock()
 if len(events) < 3 || events[0] != "queued" || events[1] != "cancelled" { t.Fatalf("unexpected job events: %#v", events) }
}
