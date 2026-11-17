package agent

import (
 "context"
 "encoding/json"
 "fmt"
 "os"
 "path/filepath"
 "strings"
 "time"

 "veltrix/internal/shared"
)

type deploymentJob struct {
 id string
 ctx context.Context
 cancel context.CancelFunc
 state string
 createdAt time.Time
}

type deploymentJobRecord struct {
 SiteID string `json:"site_id"`
 JobID string `json:"job_id"`
 State string `json:"state"`
 Progress int `json:"progress"`
 Message string `json:"message,omitempty"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
}

func validDeploymentJobID(id string) bool {
 if len(id) < 5 || len(id) > 64 || !strings.HasPrefix(id, "job-") { return false }
 for _, r := range id { if !((r >= 'a' && r <= 'f') || (r >= '0' && r <= '9') || r == '-') { return false } }
 return true
}

func (m *ProcessManager) deploymentJobPath(siteID, jobID string) (string, error) {
 if !validSiteID(siteID) || !validDeploymentJobID(jobID) { return "", fmt.Errorf("invalid deployment job identity") }
 if err := os.MkdirAll(m.DataRoot, 0700); err != nil { return "", err }
 dir, err := ensureSafeSubdirectory(m.DataRoot, filepath.Join("jobs", siteID), 0700)
 if err != nil { return "", err }
 return filepath.Join(dir, jobID+".json"), nil
}

func (m *ProcessManager) persistDeploymentJob(record deploymentJobRecord) error {
 path, err := m.deploymentJobPath(record.SiteID, record.JobID)
 if err != nil { return err }
 data, err := json.MarshalIndent(record, "", "  ")
 if err != nil { return err }
 tmp := path + ".tmp"
 if err := os.WriteFile(tmp, data, 0600); err != nil { return err }
 if err := os.Rename(tmp, path); err != nil { _ = os.Remove(tmp); return err }
 return nil
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

// BeginDeploymentJob enforces one active deployment per site and persists its queued state.
func (m *ProcessManager) BeginDeploymentJob(siteID string) (string, context.Context, error) {
 if !validSiteID(siteID) { return "", nil, fmt.Errorf("invalid site id") }
 m.jobMu.Lock()
 if m.deploymentJobs == nil { m.deploymentJobs = make(map[string]*deploymentJob) }
 if _, exists := m.deploymentJobs[siteID]; exists {
  m.jobMu.Unlock()
  return "", nil, fmt.Errorf("a deployment job is already active for this site")
 }
 created := time.Now().UTC()
 id := fmt.Sprintf("job-%x", created.UnixNano())
 ctx, cancel := context.WithCancel(context.Background())
 m.deploymentJobs[siteID] = &deploymentJob{id:id, ctx:ctx, cancel:cancel, state:"queued", createdAt:created}
 m.jobMu.Unlock()
 record := deploymentJobRecord{SiteID:siteID, JobID:id, State:"queued", Progress:0, Message:"Deployment queued", CreatedAt:created, UpdatedAt:created}
 if err := m.persistDeploymentJob(record); err != nil {
  m.jobMu.Lock(); delete(m.deploymentJobs, siteID); m.jobMu.Unlock()
  cancel()
  return "", nil, fmt.Errorf("persist deployment job: %w", err)
 }
 m.emitDeploymentJob(siteID, id, "queued", 0, "Deployment queued")
 return id, ctx, nil
}

// CancelDeploymentJob cancels only the matching active job for the requested site.
func (m *ProcessManager) CancelDeploymentJob(siteID, jobID string) error {
 if !validSiteID(siteID) || !validDeploymentJobID(jobID) { return fmt.Errorf("invalid deployment job") }
 m.jobMu.Lock()
 job, exists := m.deploymentJobs[siteID]
 if !exists || job.id != jobID {
  m.jobMu.Unlock()
  return fmt.Errorf("deployment job not found")
 }
 job.cancel()
 job.state = "cancelled"
 created := job.createdAt
 m.jobMu.Unlock()
 now := time.Now().UTC()
 _ = m.persistDeploymentJob(deploymentJobRecord{SiteID:siteID, JobID:jobID, State:"cancelled", Progress:0, Message:"Deployment cancellation requested", CreatedAt:created, UpdatedAt:now})
 m.emitDeploymentJob(siteID, jobID, "cancelled", 0, "Deployment cancellation requested")
 return nil
}

func (m *ProcessManager) UpdateDeploymentJob(siteID, jobID, state string, progress int, message string) {
 if progress < 0 { progress = 0 }; if progress > 100 { progress = 100 }
 m.jobMu.Lock()
 job, exists := m.deploymentJobs[siteID]
 active := exists && job.id == jobID && job.ctx.Err() == nil && canTransitionDeploymentJob(job.state, state)
 created := time.Time{}
 if active { job.state = state; created = job.createdAt }
 m.jobMu.Unlock()
 if !active { return }
 now := time.Now().UTC()
 _ = m.persistDeploymentJob(deploymentJobRecord{SiteID:siteID, JobID:jobID, State:state, Progress:progress, Message:message, CreatedAt:created, UpdatedAt:now})
 m.emitDeploymentJob(siteID, jobID, state, progress, message)
}

func (m *ProcessManager) FinishDeploymentJob(siteID, jobID, state, message string) {
 m.jobMu.Lock()
 job, exists := m.deploymentJobs[siteID]
 created := time.Now().UTC()
 if exists && job.id == jobID {
  created = job.createdAt
  if canTransitionDeploymentJob(job.state, state) { job.state = state }
  delete(m.deploymentJobs, siteID)
  job.cancel()
 }
 m.jobMu.Unlock()
 progress := 100
 if state == "failed" || state == "cancelled" { progress = 0 }
 now := time.Now().UTC()
 _ = m.persistDeploymentJob(deploymentJobRecord{SiteID:siteID, JobID:jobID, State:state, Progress:progress, Message:message, CreatedAt:created, UpdatedAt:now})
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
