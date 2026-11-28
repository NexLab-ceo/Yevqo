package control

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"veltrix/internal/shared"
)

const maxAgentPackageUpload = 90 << 20

var agentPackageVersionPattern = regexp.MustCompile("^[0-9][A-Za-z0-9.+:~_-]{0,63}$")
var agentPackageFilenamePattern = regexp.MustCompile("^yevqo_[0-9][A-Za-z0-9.+:~_-]{0,63}_aarch64[.]deb$")

type agentPackageManifest struct {
	Version     string    `json:"version"`
	Filename    string    `json:"filename"`
	SHA256      string    `json:"sha256"`
	Size        int64     `json:"size"`
	PublishedAt time.Time `json:"published_at"`
	DownloadURL string    `json:"download_url"`
}

func (s *Server) agentPackageDir() string {
	return filepath.Join(s.cfg.DataRoot, "updates", "agent")
}

func readAgentPackageManifest(dir string) (*agentPackageManifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, "latest.json"))
	if err != nil {
		if os.IsNotExist(err) { return nil, nil }
		return nil, err
	}
	var manifest agentPackageManifest
	if err := json.Unmarshal(data, &manifest); err != nil { return nil, err }
	if !agentPackageFilenamePattern.MatchString(manifest.Filename) || !agentPackageVersionPattern.MatchString(manifest.Version) || len(manifest.SHA256) != 64 {
		return nil, fmt.Errorf("invalid stored Agent package manifest")
	}
	manifest.DownloadURL = "/updates/agent/" + manifest.Filename
	return &manifest, nil
}

func (s *Server) handleAdminAgentPackageUpload(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		manifest, err := readAgentPackageManifest(s.agentPackageDir())
		if err != nil { apiError(w, shared.ErrInternal("could not read Agent package metadata")); return }
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "package": manifest})
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, maxAgentPackageUpload)
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			apiError(w, shared.ErrBadRequest("Agent .deb upload is invalid or exceeds the 90 MiB limit"))
			return
		}
		file, _, err := r.FormFile("package")
		if err != nil { apiError(w, shared.ErrBadRequest("select an Agent .deb package")); return }
		defer file.Close()

		dir := s.agentPackageDir()
		if err := os.MkdirAll(dir, 0700); err != nil { apiError(w, shared.ErrInternal("could not create Agent package storage")); return }
		tmp, err := os.CreateTemp(dir, ".agent-upload-*.deb")
		if err != nil { apiError(w, shared.ErrInternal("could not stage Agent package")); return }
		tmpName := tmp.Name()
		defer os.Remove(tmpName)
		hash := sha256.New()
		size, copyErr := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(file, maxAgentPackageUpload+1))
		closeErr := tmp.Close()
		if copyErr != nil || closeErr != nil || size == 0 || size > maxAgentPackageUpload {
			apiError(w, shared.ErrBadRequest("Agent package upload failed or is too large"))
			return
		}
		packageName, err := debField(tmpName, "Package")
		if err != nil || packageName != "yevqo" { apiError(w, shared.ErrBadRequest("Debian package must have Package: yevqo (Agent only)")); return }
		architecture, err := debField(tmpName, "Architecture")
		if err != nil || architecture != "aarch64" { apiError(w, shared.ErrBadRequest("Agent package must target Architecture: aarch64")); return }
		version, err := debField(tmpName, "Version")
		if err != nil || !agentPackageVersionPattern.MatchString(version) { apiError(w, shared.ErrBadRequest("Agent package has an invalid Debian Version")); return }

		manifest, err := readAgentPackageManifest(dir)
		if err != nil { apiError(w, shared.ErrInternal("could not read current Agent package version")); return }
		checksum := hex.EncodeToString(hash.Sum(nil))
		filename := "yevqo_" + version + "_aarch64.deb"
		if manifest != nil {
			if version == manifest.Version && checksum == manifest.SHA256 {
				writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "package": manifest})
				return
			}
			if version == manifest.Version || !debianVersionGreater(version, manifest.Version) {
				apiError(w, shared.ErrConflict("Agent package version must be newer than the currently published version"))
				return
			}
		}
		finalPath := filepath.Join(dir, filename)
		if _, err := os.Stat(finalPath); err == nil {
			apiError(w, shared.ErrConflict("this Agent package version already exists; versions are immutable"))
			return
		}
		if err := os.Rename(tmpName, finalPath); err != nil { apiError(w, shared.ErrInternal("could not publish Agent package file")); return }
		published := agentPackageManifest{Version: version, Filename: filename, SHA256: checksum, Size: size, PublishedAt: time.Now().UTC(), DownloadURL: "/updates/agent/" + filename}
		data, err := json.MarshalIndent(published, "", "  ")
		if err != nil { _ = os.Remove(finalPath); apiError(w, shared.ErrInternal("could not encode Agent package metadata")); return }
		metaTmp, err := os.CreateTemp(dir, ".latest-*.json")
		if err != nil { _ = os.Remove(finalPath); apiError(w, shared.ErrInternal("could not stage Agent package metadata")); return }
		metaName := metaTmp.Name()
		if _, err = metaTmp.Write(data); err == nil { err = metaTmp.Sync() }
		if closeErr := metaTmp.Close(); err == nil { err = closeErr }
		if err == nil { err = os.Rename(metaName, filepath.Join(dir, "latest.json")) }
		_ = os.Remove(metaName)
		if err != nil { _ = os.Remove(finalPath); apiError(w, shared.ErrInternal("could not publish Agent package metadata")); return }
		s.logAdminAction(r, "agent_package_published", 0)
		writeJSON(w, http.StatusCreated, map[string]interface{}{"ok": true, "package": published})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func debField(path, field string) (string, error) {
	out, err := exec.Command("dpkg-deb", "--field", path, field).Output()
	if err != nil { return "", err }
	return strings.TrimSpace(string(out)), nil
}

func debianVersionGreater(candidate, current string) bool {
	return exec.Command("dpkg", "--compare-versions", candidate, "gt", current).Run() == nil
}

func (s *Server) handleAgentPackageUpdates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	dir := s.agentPackageDir()
	manifest, err := readAgentPackageManifest(dir)
	if err != nil { apiError(w, shared.ErrInternal("could not read Agent package metadata")); return }
	name := strings.TrimPrefix(r.URL.Path, "/updates/agent/")
	if name == "" || name == "latest.json" {
		if manifest == nil { http.NotFound(w, r); return }
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, manifest)
		return
	}
	if !agentPackageFilenamePattern.MatchString(name) { http.NotFound(w, r); return }
	path := filepath.Join(dir, name)
	file, err := os.Open(path)
	if err != nil { http.NotFound(w, r); return }
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() { http.NotFound(w, r); return }
	w.Header().Set("Content-Type", "application/vnd.debian.binary-package")
	w.Header().Set("Content-Disposition", "attachment; filename="+name)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeContent(w, r, name, info.ModTime(), file)
}
