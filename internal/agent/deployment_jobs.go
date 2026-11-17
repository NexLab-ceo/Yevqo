package agent

import (
 "context"
 "fmt"
 "time"

 "veltrix/internal/shared"
)

type deploymentJob struct {
 id string
 ctx context.Context
 cancel context.CancelFunc
 state string
}

// SetDeploymentEmitter installs the Agent-to-Control-Plane progress callback.
func (m *ProcessManager) SetDeploymentEmitter(emit func(shared.DeploymentJobEvent)) {
 m.jobMu.Lock()
 m.deploymentEmitter = emit
 m.jobMu.Unlock()
}

func (m *ProcessManager) emitDeploymentJob(siteID, jobID, state string, progress int, message string) {
 m.jobMu.Lock()
 emit := m.deploymentEmitter
 m.jobMu.Unlock()
 if emit == nil { return }
 emit(shared.DeploymentJobEvent{SiteId:siteID, JobId:jobID, State:state, Progress:progress, Message:message, Timestamp:time.Now().UTC()})
}

// BeginDeploymentJob enforces one active deployment per site.
func (m *ProcessManager) BeginDeploymentJob(siteID string) (string, context.Context, error) {
 if !validSiteID(siteID) { return "", nil, fmt.Errorf("invalid site id") }
 m.jobMu.Lock()
 if m.deploymentJobs == nil { m.deploymentJobs = make(map[string]*deploymentJob) }
 if _, exists := m.deploymentJobs[siteID]; exists {
  m.jobMu.Unlock()
  return "", nil, fmt.Errorf("a deployment job is already active for this site")
 }
 id := fmt.Sprintf("job-%x", time.Now().UnixNano())
 ctx, cancel := context.WithCancel(context.Background())
 m.deploymentJobs[siteID] = &deploymentJob{id:id, ctx:ctx, cancel:cancel, state:"queued"}
 m.jobMu.Unlock()
 m.emitDeploymentJob(siteID, id, "queued", 0, "Deployment queued")
 return id, ctx, nil
}

// CancelDeploymentJob cancels only the matching active job for the requested site.
func (m *ProcessManager) CancelDeploymentJob(siteID, jobID string) error {
 if !validSiteID(siteID) || jobID == "" { return fmt.Errorf("invalid deployment job") }
 m.jobMu.Lock()
 job, exists := m.deploymentJobs[siteID]
 if !exists || job.id != jobID {
  m.jobMu.Unlock()
  return fmt.Errorf("deployment job not found")
 }
 job.cancel()
 job.state = "cancelled"
 m.jobMu.Unlock()
 m.emitDeploymentJob(siteID, jobID, "cancelled", 0, "Deployment cancellation requested")
 return nil
}

func (m *ProcessManager) UpdateDeploymentJob(siteID, jobID, state string, progress int, message string) {
 if progress < 0 { progress = 0 }; if progress > 100 { progress = 100 }
 m.jobMu.Lock()
 job, exists := m.deploymentJobs[siteID]
 active := exists && job.id == jobID && job.ctx.Err() == nil && canTransitionDeploymentJob(job.state, state)
 if active { job.state = state }
 m.jobMu.Unlock()
 if active { m.emitDeploymentJob(siteID, jobID, state, progress, message) }
}

func (m *ProcessManager) FinishDeploymentJob(siteID, jobID, state, message string) {
 m.jobMu.Lock()
 job, exists := m.deploymentJobs[siteID]
 if exists && job.id == jobID {
  if canTransitionDeploymentJob(job.state, state) { job.state = state }
  delete(m.deploymentJobs, siteID)
  job.cancel()
 }
 m.jobMu.Unlock()
 progress := 100
 if state == "failed" || state == "cancelled" { progress = 0 }
 m.emitDeploymentJob(siteID, jobID, state, progress, message)
}


func canTransitionDeploymentJob(current, next string) bool {
 if current == next { return true }
 if next == "failed" || next == "cancelled" { return current != "healthy" && current != "failed" && current != "cancelled" }
 allowed := map[string][]string{
  "queued":{"preparing"},
  "preparing":{"cloning"},
  "cloning":{"installing","starting"},
  "installing":{"building"},
  "building":{"starting"},
  "starting":{"healthy"},
 }
 for _, candidate := range allowed[current] { if candidate == next { return true } }
 return false
}
