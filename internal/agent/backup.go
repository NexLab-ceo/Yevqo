package agent

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"veltrix/internal/shared"
)

func createSiteBackup(dataRoot, siteID string) (*shared.SiteBackupResult, error) {
	siteDir, err := safeSiteDirectory(dataRoot, siteID)
	if err != nil { return nil, err }
	info, err := os.Stat(siteDir)
	if err != nil || !info.IsDir() { return nil, fmt.Errorf("site directory not found") }
	prefix := os.Getenv("PREFIX")
	if prefix == "" { return nil, fmt.Errorf("Termux PREFIX is not set") }
	backupDir := filepath.Join(prefix, "var", "lib", "veltrix-agent", "backups", "sites", siteID)
	if err := os.MkdirAll(backupDir, 0o700); err != nil { return nil, fmt.Errorf("create backup directory: %w", err) }
	rootResolved, rootErr := filepath.EvalSymlinks(prefix)
	backupResolved, backupErr := filepath.EvalSymlinks(backupDir)
	if rootErr != nil || backupErr != nil { return nil, fmt.Errorf("could not verify backup directory") }
	relBackup, relErr := filepath.Rel(rootResolved, backupResolved)
	if relErr != nil || relBackup == ".." || strings.HasPrefix(relBackup, ".."+string(filepath.Separator)) { return nil, fmt.Errorf("backup directory resolves outside the agent data directory") }
	name := siteID + "-" + time.Now().UTC().Format("20060102T150405.000000Z") + ".tar.gz"
	fullPath := filepath.Join(backupDir, name)
	file, err := os.OpenFile(fullPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil { return nil, fmt.Errorf("create backup file: %w", err) }
	ok := false
	defer func(){ if !ok { _ = file.Close(); _ = os.Remove(fullPath) } }()
	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	walkErr := filepath.WalkDir(siteDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil { return walkErr }
		if path == siteDir { return nil }
		if entry.Type()&os.ModeSymlink != 0 { return nil }
		rel, err := filepath.Rel(siteDir, path)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) { return fmt.Errorf("invalid archive path") }
		if rel == ".yevqo" || strings.HasPrefix(rel, ".yevqo"+string(filepath.Separator)) {
			if entry.IsDir() { return filepath.SkipDir }
			return nil
		}
		info, err := entry.Info()
		if err != nil { return err }
		header, err := tar.FileInfoHeader(info, "")
		if err != nil { return err }
		header.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(header); err != nil { return err }
		if !info.Mode().IsRegular() { return nil }
		src, err := os.Open(path)
		if err != nil { return err }
		_, copyErr := io.Copy(tw, src)
		closeErr := src.Close()
		if copyErr != nil { return copyErr }
		return closeErr
	})
	if walkErr != nil { _ = tw.Close(); _ = gz.Close(); return nil, fmt.Errorf("archive site: %w", walkErr) }
	if err := tw.Close(); err != nil { _ = gz.Close(); return nil, err }
	if err := gz.Close(); err != nil { return nil, err }
	if err := file.Sync(); err != nil { return nil, err }
	if err := file.Close(); err != nil { return nil, err }
	ok = true
	st, err := os.Stat(fullPath)
	if err != nil { return nil, err }
	return &shared.SiteBackupResult{FileName:name,SizeBytes:st.Size()},nil
}
