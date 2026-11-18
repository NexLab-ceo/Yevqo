package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BuildHistoryRecord is a private, device-local snapshot of a completed build.
type BuildHistoryRecord struct {
	ID          string             `json:"id"`
	SiteID      string             `json:"site_id"`
	OutputBytes int                `json:"output_bytes"`
	Result      AndroidBuildResult `json:"result"`
}

const buildHistoryRetention = 10

func buildHistoryDirectory(dataRoot, siteID string) (string, error) {
	if !validSiteID(siteID) {
		return "", fmt.Errorf("invalid site id")
	}
	if err := os.MkdirAll(dataRoot, 0o700); err != nil {
		return "", fmt.Errorf("prepare Agent data directory: %w", err)
	}
	root, err := ensureSafeSubdirectory(dataRoot, "build-history", 0o700)
	if err != nil {
		return "", fmt.Errorf("prepare build history directory: %w", err)
	}
	return ensureSafeSubdirectory(root, siteID, 0o700)
}

func persistBuildHistory(dataRoot, siteID string, result AndroidBuildResult) error {
	dir, err := buildHistoryDirectory(dataRoot, siteID)
	if err != nil {
		return err
	}
	finished := result.FinishedAt
	if finished.IsZero() {
		finished = time.Now().UTC()
	}
	id := finished.UTC().Format("20060102T150405.000000000Z")
	record := BuildHistoryRecord{ID: id, SiteID: siteID, OutputBytes: len(result.Output), Result: result}
	encoded, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode build history: %w", err)
	}
	target := filepath.Join(dir, id+".json")
	temp := filepath.Join(dir, "."+id+".tmp")
	file, err := os.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create build history record: %w", err)
	}
	if _, err = file.Write(encoded); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("write build history record: %w", err)
	}
	if closeErr != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("close build history record: %w", closeErr)
	}
	if err := os.Rename(temp, target); err != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("finalize build history record: %w", err)
	}
	return pruneBuildHistory(dir, buildHistoryRetention)
}

func pruneBuildHistory(dir string, keep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var names []string
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	if len(names) <= keep {
		return nil
	}
	for _, name := range names[:len(names)-keep] {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("prune build history %s: %w", name, err)
		}
	}
	return nil
}

// ListBuildHistory returns newest-first local records for one site.
func ListBuildHistory(dataRoot, siteID string, limit int) ([]BuildHistoryRecord, error) {
	dir, err := buildHistoryDirectory(dataRoot, siteID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > buildHistoryRetention {
		limit = buildHistoryRetention
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, entry.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	if len(names) > limit {
		names = names[:limit]
	}
	records := make([]BuildHistoryRecord, 0, len(names))
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("read build history %s: %w", name, err)
		}
		var record BuildHistoryRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("decode build history %s: %w", name, err)
		}
		if record.SiteID != siteID {
			return nil, fmt.Errorf("build history record %s has mismatched site id", name)
		}
		record.OutputBytes = len(record.Result.Output)
		record.Result.Output = ""
		records = append(records, record)
	}
	return records, nil
}

// GetBuildHistory returns one full record, including its stored log output.
func GetBuildHistory(dataRoot, siteID, id string) (BuildHistoryRecord, error) {
	if !validSiteID(siteID) {
		return BuildHistoryRecord{}, fmt.Errorf("invalid site id")
	}
	if _, err := time.Parse("20060102T150405.000000000Z", id); err != nil {
		return BuildHistoryRecord{}, fmt.Errorf("invalid build history id")
	}
	dir, err := buildHistoryDirectory(dataRoot, siteID)
	if err != nil {
		return BuildHistoryRecord{}, err
	}
	path := filepath.Join(dir, id+".json")
	info, err := os.Lstat(path)
	if err != nil {
		return BuildHistoryRecord{}, fmt.Errorf("build history record not found: %w", err)
	}
	if !info.Mode().IsRegular() {
		return BuildHistoryRecord{}, fmt.Errorf("build history record is not a regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return BuildHistoryRecord{}, err
	}
	var record BuildHistoryRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return BuildHistoryRecord{}, fmt.Errorf("decode build history record: %w", err)
	}
	if record.SiteID != siteID || record.ID != id {
		return BuildHistoryRecord{}, fmt.Errorf("build history record identity mismatch")
	}
	record.OutputBytes = len(record.Result.Output)
	return record, nil
}
