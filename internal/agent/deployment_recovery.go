package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// deploymentTransaction is a write-ahead record. It is durable before the
// previous process is stopped, so Agent startup can restore a known-good run.
type deploymentTransaction struct {
	Version        int           `json:"version"`
	SiteID         string        `json:"site_id"`
	State          string        `json:"state"`
	CandidateCommit string       `json:"candidate_commit,omitempty"`
	HadPrevious    bool          `json:"had_previous"`
	Previous       ProcessRecord `json:"previous,omitempty"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

func (m *ProcessManager) deploymentTransactionPath(siteID string) (string, error) {
	if !validSiteID(siteID) {
		return "", fmt.Errorf("invalid deployment transaction site")
	}
	siteRoot, err := safeSiteDirectory(m.DataRoot, siteID)
	if err != nil {
		return "", err
	}
	metadata, err := ensureSafeSubdirectory(siteRoot, ".yevqo", 0o700)
	if err != nil {
		return "", err
	}
	return filepath.Join(metadata, "deployment-transaction.json"), nil
}

// createDeploymentTransaction refuses to replace an unresolved recovery record.
func (m *ProcessManager) createDeploymentTransaction(tx deploymentTransaction) error {
	_, exists, err := m.readDeploymentTransaction(tx.SiteID)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("an unresolved deployment recovery transaction already exists")
	}
	return m.persistDeploymentTransaction(tx)
}

func (m *ProcessManager) persistDeploymentTransaction(tx deploymentTransaction) error {
	if tx.Version != 1 || !validSiteID(tx.SiteID) {
		return fmt.Errorf("invalid deployment transaction")
	}
	path, err := m.deploymentTransactionPath(tx.SiteID)
	if err != nil {
		return err
	}
	tx.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(tx, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write deployment transaction: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("commit deployment transaction: %w", err)
	}
	return syncDirectory(filepath.Dir(path))
}

func (m *ProcessManager) removeDeploymentTransaction(siteID string) error {
	path, err := m.deploymentTransactionPath(siteID)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil && !strings.Contains(strings.ToLower(err.Error()), "invalid argument") {
		return err
	}
	return nil
}

func (m *ProcessManager) readDeploymentTransaction(siteID string) (deploymentTransaction, bool, error) {
	path, err := m.deploymentTransactionPath(siteID)
	if err != nil {
		return deploymentTransaction{}, false, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return deploymentTransaction{}, false, nil
	}
	if err != nil {
		return deploymentTransaction{}, false, err
	}
	var tx deploymentTransaction
	if err := json.Unmarshal(data, &tx); err != nil {
		return deploymentTransaction{}, false, fmt.Errorf("decode deployment transaction: %w", err)
	}
	if tx.Version != 1 || tx.SiteID != siteID || !validSiteID(tx.SiteID) ||
		(tx.State != "activating" && tx.State != "committed") ||
		(tx.HadPrevious && (tx.Previous.SiteID != siteID || tx.Previous.PID <= 0 || tx.Previous.ProcessGroup != tx.Previous.PID || tx.Previous.StartToken == "")) {
		return deploymentTransaction{}, false, fmt.Errorf("deployment transaction failed validation")
	}
	if tx.CandidateCommit != "" && !validReleaseCommit(tx.CandidateCommit) {
		return deploymentTransaction{}, false, fmt.Errorf("deployment transaction has invalid candidate release")
	}
	return tx, true, nil
}

// recoverDeploymentTransactions runs before process adoption. Any transaction
// not durably marked committed is rolled back to its previous process record.
func (m *ProcessManager) recoverDeploymentTransactions() {
	sitesRoot := filepath.Join(m.DataRoot, "sites")
	entries, err := os.ReadDir(sitesRoot)
	if err != nil {
		if !os.IsNotExist(err) {
			m.emitProcessLog("agent", fmt.Sprintf("deployment recovery could not read sites directory: %v", err))
		}
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || !validSiteID(entry.Name()) {
			continue
		}
		siteID := entry.Name()
		tx, exists, readErr := m.readDeploymentTransaction(siteID)
		if readErr != nil {
			m.emitProcessLog(siteID, fmt.Sprintf("deployment recovery is blocked by an invalid transaction record: %v", readErr))
			continue
		}
		if !exists {
			continue
		}
		if tx.State == "committed" {
			if err := m.removeDeploymentTransaction(siteID); err != nil {
				m.emitProcessLog(siteID, fmt.Sprintf("could not finalize deployment transaction cleanup: %v", err))
			}
			continue
		}
		if err := m.rollbackDeploymentTransaction(tx); err != nil {
			m.emitProcessLog(siteID, fmt.Sprintf("deployment recovery failed; transaction retained for retry: %v", err))
			continue
		}
		if err := m.removeDeploymentTransaction(siteID); err != nil {
			m.emitProcessLog(siteID, fmt.Sprintf("deployment recovered but transaction cleanup failed: %v", err))
			continue
		}
		m.emitProcessLog(siteID, "interrupted deployment was rolled back to the last confirmed release")
	}
}

func (m *ProcessManager) rollbackDeploymentTransaction(tx deploymentTransaction) error {
	if m.registry == nil || m.registryErr != nil {
		return fmt.Errorf("process registry unavailable: %v", m.registryErr)
	}
	current, hasCurrent := m.registry.Get(tx.SiteID)
	if hasCurrent {
		isPrevious := tx.HadPrevious && current.PID == tx.Previous.PID && current.StartToken == tx.Previous.StartToken
		if !isPrevious {
			if verifyProcessRecord(current) {
				if err := m.stopSiteLocked(tx.SiteID); err != nil {
					return fmt.Errorf("stop unconfirmed candidate process: %w", err)
				}
			} else if err := m.registry.Delete(tx.SiteID); err != nil {
				return fmt.Errorf("remove stale candidate process record: %w", err)
			}
		}
	}
	if !tx.HadPrevious {
		return nil
	}
	if current, ok := m.registry.Get(tx.SiteID); ok && current.PID == tx.Previous.PID &&
		current.StartToken == tx.Previous.StartToken && verifyProcessRecord(current) {
		return nil
	}
	if verifyProcessRecord(tx.Previous) {
		if err := m.registry.Save(tx.Previous); err != nil {
			return fmt.Errorf("restore previous process record: %w", err)
		}
		return nil
	}
	runPath := tx.Previous.RunPath
	if runPath == "" {
		siteRoot, err := safeSiteDirectory(m.DataRoot, tx.SiteID)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(siteRoot, tx.Previous.WorkingDir)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return fmt.Errorf("previous process working directory is outside site root")
		}
		runPath = filepath.Join("sites", tx.SiteID, rel)
	}
	if _, err := m.startSiteWithCommandLocked(tx.SiteID, runPath, tx.Previous.Port,
		tx.Previous.Runtime, nil, tx.Previous.StartCommand); err != nil {
		return fmt.Errorf("restart previous release: %w", err)
	}
	restored, ok := m.registry.Get(tx.SiteID)
	if !ok {
		return fmt.Errorf("previous release started but its process record is missing")
	}
	restored.RestartPolicy = tx.Previous.RestartPolicy
	restored.RestartCount = tx.Previous.RestartCount
	restored.LastRestartAt = tx.Previous.LastRestartAt
	if err := m.registry.Save(restored); err != nil {
		return fmt.Errorf("persist restored process policy: %w", err)
	}
	return nil
}
