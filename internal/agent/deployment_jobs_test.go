package agent

import (
 "sync"
 "testing"

 "veltrix/internal/shared"
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


func TestDeploymentJobStateMachineRejectsSkippedAndTerminalTransitions(t *testing.T) {
 valid := [][2]string{
  {"queued","preparing"}, {"preparing","cloning"}, {"cloning","installing"},
  {"installing","building"}, {"building","starting"}, {"starting","healthy"},
  {"queued","failed"}, {"building","cancelled"},
 }
 for _, pair := range valid {
  if !canTransitionDeploymentJob(pair[0], pair[1]) { t.Errorf("transition %s -> %s should be valid", pair[0], pair[1]) }
 }
 invalid := [][2]string{
  {"queued","building"}, {"cloning","healthy"}, {"healthy","failed"},
  {"failed","starting"}, {"cancelled","preparing"},
 }
 for _, pair := range invalid {
  if canTransitionDeploymentJob(pair[0], pair[1]) { t.Errorf("transition %s -> %s should be rejected", pair[0], pair[1]) }
 }
}
