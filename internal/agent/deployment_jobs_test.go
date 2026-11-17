package agent

import (
 "encoding/json"
 "os"
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


func TestDeploymentJobStatePersistsToDisk(t *testing.T) {
 root := t.TempDir()
 manager := &ProcessManager{DataRoot:root}
 jobID, _, err := manager.BeginDeploymentJob("persisted")
 if err != nil { t.Fatal(err) }
 path, err := manager.deploymentJobPath("persisted", jobID)
 if err != nil { t.Fatal(err) }
 read := func() deploymentJobRecord {
  raw, err := os.ReadFile(path); if err != nil { t.Fatal(err) }
  var record deploymentJobRecord
  if err := json.Unmarshal(raw, &record); err != nil { t.Fatal(err) }
  return record
 }
 if record := read(); record.State != "queued" || record.JobID != jobID { t.Fatalf("unexpected queued record: %#v", record) }
 manager.UpdateDeploymentJob("persisted", jobID, "preparing", 5, "Preparing")
 manager.FinishDeploymentJob("persisted", jobID, "failed", "simulated failure")
 if record := read(); record.State != "failed" || record.Progress != 0 { t.Fatalf("unexpected terminal record: %#v", record) }
}


func TestCancelledDeploymentCannotBeOverwrittenByLateSuccess(t *testing.T) {
 root := t.TempDir()
 manager := &ProcessManager{DataRoot:root}
 var mu sync.Mutex
 var states []string
 manager.SetDeploymentEmitter(func(event shared.DeploymentJobEvent) {
  mu.Lock()
  states = append(states, event.State)
  mu.Unlock()
 })
 jobID, ctx, err := manager.BeginDeploymentJob("cancel-race")
 if err != nil { t.Fatal(err) }
 if err := manager.CancelDeploymentJob("cancel-race", jobID); err != nil { t.Fatal(err) }
 if ctx.Err() == nil { t.Fatal("cancellation did not cancel deployment context") }

 // Simulate a deploy routine returning success after cancellation won the race.
 manager.FinishDeploymentJob("cancel-race", jobID, "healthy", "Deployment completed")
 path, err := manager.deploymentJobPath("cancel-race", jobID)
 if err != nil { t.Fatal(err) }
 raw, err := os.ReadFile(path)
 if err != nil { t.Fatal(err) }
 var record deploymentJobRecord
 if err := json.Unmarshal(raw, &record); err != nil { t.Fatal(err) }
 if record.State != "cancelled" || record.Progress != 0 {
  t.Fatalf("late success overwrote cancellation: state=%q progress=%d", record.State, record.Progress)
 }
 mu.Lock()
 defer mu.Unlock()
 if len(states) < 3 || states[len(states)-1] != "cancelled" {
  t.Fatalf("final event must remain cancelled, got %#v", states)
 }
}

func TestFinishDeploymentJobIgnoresStaleJobID(t *testing.T) {
 root := t.TempDir()
 manager := &ProcessManager{DataRoot:root}
 jobID, _, err := manager.BeginDeploymentJob("stale")
 if err != nil { t.Fatal(err) }
 manager.FinishDeploymentJob("stale", "job-deadbeef", "healthy", "stale")
 if _, ok := manager.deploymentJobs["stale"]; !ok { t.Fatal("stale completion removed the active job") }
 manager.FinishDeploymentJob("stale", jobID, "failed", "expected failure")
}
