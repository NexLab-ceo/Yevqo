package agent

import (
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
	"strconv"
	"time"
)

// watchProcesses reconciles the durable registry and removes stale identities.
func (m *ProcessManager) watchProcesses() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if m.registry == nil { return }
		for _, record := range m.registry.List() {
			if verifyProcessRecord(record) { continue }
			m.mu.RLock()
			managed, tracked := m.procs[record.SiteID]
			tracked = tracked && managed.PID == record.PID
			m.mu.RUnlock()
			if tracked { continue } // the waiter owns crash recovery for this child
			m.mu.Lock()
			delete(m.procs, record.SiteID)
			m.mu.Unlock()
			// Never signal a PID whose identity no longer matches. A stale record
			// is safe to remove, but automatic recovery is handled by the child waiter.
			_ = m.registry.Delete(record.SiteID)
			if record.RestartPolicy != "never" {
				go m.recoverSite(record)
			}
		}
	}
}

// IsSiteHealthy reports process identity and local TCP readiness.
func (m *ProcessManager) IsSiteHealthy(siteID string) bool {
	if !validSiteID(siteID) || m.registry == nil { return false }
	r, ok := m.registry.Get(siteID)
	if !ok || !verifyProcessRecord(r) { return false }
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(r.Port)), 500*time.Millisecond)
	if err != nil { return false }
	_ = conn.Close()
	return true
}

// waitForProcessExit reaps a child and starts bounded recovery only for an
// unexpected exit. Record identity prevents an old waiter touching a new run.
func (m *ProcessManager) waitForProcessExit(siteID string, pid int, startToken string, cmd *exec.Cmd) {
	waitErr := cmd.Wait()
	m.mu.Lock()
	if current, ok := m.procs[siteID]; ok && current.PID == pid { delete(m.procs, siteID) }
	m.mu.Unlock()
	if m.registry == nil { return }
	record, ok := m.registry.Get(siteID)
	if !ok || record.PID != pid || record.StartToken != startToken { return }

	m.restartMu.Lock()
	intentional := m.intentionalStops[siteID]
	m.restartMu.Unlock()
	_ = m.registry.Delete(siteID)
	if intentional { return }
	reason := "process exited unexpectedly"
	if waitErr != nil { reason = fmt.Sprintf("process exited unexpectedly: %v", waitErr) }
	m.emitProcessLog(siteID, reason)
	m.emitProcessStatus(siteID, "error", 0)
	if shouldRestart(record.RestartPolicy, waitErr) { go m.recoverSite(record) }
}

// recoverSite retries the last known immutable/run path with exponential
// backoff. Attempts are bounded per site and reset by an explicit StopSite.
func (m *ProcessManager) recoverSite(record ProcessRecord) {
	for {
		m.restartMu.Lock()
		if m.restartAttempts == nil { m.restartAttempts = make(map[string]int) }
		if m.intentionalStops == nil { m.intentionalStops = make(map[string]bool) }
		attempt := m.restartAttempts[record.SiteID] + 1
		if attempt > maxAutomaticRestarts {
			m.restartMu.Unlock()
			m.emitProcessLog(record.SiteID, "automatic restart limit reached; manual intervention required")
			return
		}
		m.restartAttempts[record.SiteID] = attempt
		m.restartMu.Unlock()

		delay := restartDelay(attempt)
		m.emitProcessLog(record.SiteID, fmt.Sprintf("unexpected exit; automatic restart %d/%d in %s", attempt, maxAutomaticRestarts, delay))
		time.Sleep(delay)

		// A manual start/stop or a newer deployment supersedes this recovery.
		m.restartMu.Lock()
		stopped := m.intentionalStops[record.SiteID]
		m.restartMu.Unlock()
		if stopped { return }
		if current, exists := m.registry.Get(record.SiteID); exists && verifyProcessRecord(current) { return }

		runPath := record.RunPath
		if runPath == "" {
			siteRoot, err := safeSiteDirectory(m.DataRoot, record.SiteID)
			if err != nil { m.emitProcessLog(record.SiteID, err.Error()); return }
			rel, err := filepath.Rel(siteRoot, record.WorkingDir)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				m.emitProcessLog(record.SiteID, "recovery aborted: previous working directory is outside site root")
			 return
			}
			runPath = filepath.Join("sites", record.SiteID, rel)
		}
		startPort := record.Port
		if record.BackendPort > 0 { startPort = record.BackendPort }
		if _, err := m.StartSiteWithCommand(record.SiteID, runPath, startPort, record.Runtime, record.StartCommand); err != nil {
			m.emitProcessLog(record.SiteID, fmt.Sprintf("automatic restart %d failed: %v", attempt, err))
			continue
		}
		if current, ok := m.registry.Get(record.SiteID); ok {
			current.Port = record.Port
			current.BackendPort = record.BackendPort
			current.RestartPolicy = record.RestartPolicy
			if err := m.registry.Save(current); err != nil {
				m.emitProcessLog(record.SiteID, fmt.Sprintf("could not persist recovered backend identity: %v", err))
				continue
			}
			m.mu.Lock()
			if process := m.procs[record.SiteID]; process != nil && process.PID == current.PID { process.Port = current.Port }
			m.mu.Unlock()
			if current.BackendPort > 0 {
				if err := m.ensureDeploymentProxy(record.SiteID, current.Port, current.BackendPort); err != nil {
					m.emitProcessLog(record.SiteID, fmt.Sprintf("could not restore stable listener after restart: %v", err))
					continue
				}
			}
		}
		m.emitProcessLog(record.SiteID, fmt.Sprintf("site recovered successfully (attempt %d)", attempt))
		if current, ok := m.registry.Get(record.SiteID); ok { m.emitProcessStatus(record.SiteID, "running", current.PID) }
		return
	}
}

func (m *ProcessManager) emitProcessLog(siteID, line string) {
	m.mu.RLock()
	emit := m.logEmitter
	m.mu.RUnlock()
	if emit != nil { emit(siteID, line) }
}

func (m *ProcessManager) emitProcessStatus(siteID, status string, pid int) {
	m.mu.RLock()
	emit := m.statusEmitter
	m.mu.RUnlock()
	if emit != nil { emit(siteID, status, pid) }
}
