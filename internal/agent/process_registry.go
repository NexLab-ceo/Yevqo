package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ProcessRecord is the durable identity of a managed application process.
// PID alone is never sufficient to signal a process after an Agent restart.
type ProcessRecord struct {
	SiteID      string    `json:"site_id"`
	PID          int       `json:"pid"`
	ProcessGroup int       `json:"process_group"`
	Port         int       `json:"port"`
	Runtime      string    `json:"runtime"`
	WorkingDir   string    `json:"working_dir"`
	LogPath      string    `json:"log_path"`
	StartedAt    time.Time `json:"started_at"`
	StartToken   string    `json:"start_token"`
}

// ProcessRegistry stores process metadata atomically on disk.
type ProcessRegistry struct {
	mu   sync.Mutex
	path string
	rows map[string]ProcessRecord
}

func NewProcessRegistry(dataRoot string) (*ProcessRegistry, error) {
	r := &ProcessRegistry{
		path: filepath.Join(dataRoot, "processes.json"),
		rows: make(map[string]ProcessRecord),
	}
	if err := r.load(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *ProcessRegistry) load() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	data, err := os.ReadFile(r.path)
	if os.IsNotExist(err) { return nil }
	if err != nil { return err }
	var rows map[string]ProcessRecord
	if err := json.Unmarshal(data, &rows); err != nil {
		return fmt.Errorf("decode process registry: %w", err)
	}
	if rows != nil { r.rows = rows }
	return nil
}

func (r *ProcessRegistry) Save(record ProcessRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if record.SiteID == "" || record.PID <= 0 || record.ProcessGroup <= 0 {
		return fmt.Errorf("invalid process record")
	}
	r.rows[record.SiteID] = record
	return r.persistLocked()
}

func (r *ProcessRegistry) Delete(siteID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.rows, siteID)
	return r.persistLocked()
}

func (r *ProcessRegistry) Get(siteID string) (ProcessRecord, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.rows[siteID]
	return row, ok
}

func (r *ProcessRegistry) List() []ProcessRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]ProcessRecord, 0, len(r.rows))
	for _, row := range r.rows { out = append(out, row) }
	return out
}

func (r *ProcessRegistry) persistLocked() error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0700); err != nil { return err }
	data, err := json.MarshalIndent(r.rows, "", "  ")
	if err != nil { return err }
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil { return err }
	if err := os.Rename(tmp, r.path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
