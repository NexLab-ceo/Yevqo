package control

import (
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"veltrix/internal/shared"
)

var gitBranchPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,127}$`)

type siteGitSettings struct {
	GitURL       string
	GitBranch    string
	BuildCommand string
	RootDir      string
	HealthCheckPath string
	CloneTimeoutSeconds int
	BuildTimeoutSeconds int
	HealthCheckTimeoutSeconds int
	StartCommand string
	GitToken     string
	Commit       string
	DeployedAt   sql.NullTime
}

type siteGitInput struct {
	RepositoryURL string `json:"repository_url"`
	Branch        string `json:"branch"`
	BuildCommand  string `json:"build_command"`
	RootDirectory string `json:"root_directory"`
	HealthCheckPath string `json:"health_check_path"`
	CloneTimeoutSeconds int `json:"clone_timeout_seconds"`
	BuildTimeoutSeconds int `json:"build_timeout_seconds"`
	HealthCheckTimeoutSeconds int `json:"health_check_timeout_seconds"`
	StartCommand string `json:"start_command"`
	GitHubToken   string `json:"github_token"`
}

func validateGitRepository(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.User != nil || strings.ContainsAny(raw, "\r\n") {
		return false
	}
	return u.Scheme == "https" || u.Scheme == "ssh"
}

func validateSiteRootDir(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" { raw = "." }
	if strings.Contains(raw, "\\") || path.IsAbs(raw) { return "", false }
	clean := path.Clean(raw)
	if clean == ".." || strings.HasPrefix(clean, "../") { return "", false }
	return clean, true
}

func (s *Server) siteGitSettings(ctx context.Context, deviceID int64, siteID string) (*siteGitSettings, error) {
	var out siteGitSettings
	var gitURL, commit sql.NullString
	var encrypted []byte
	err := s.db.conn.QueryRowContext(ctx, `SELECT git_url,git_branch,build_command,root_dir,health_check_path,clone_timeout_seconds,build_timeout_seconds,health_check_timeout_seconds,start_command,git_token_enc,deployed_commit,deployed_at
		FROM sites WHERE device_id=? AND site_id=?`, deviceID, siteID).Scan(
		&gitURL, &out.GitBranch, &out.BuildCommand, &out.RootDir, &out.HealthCheckPath, &out.StartCommand, &encrypted, &commit, &out.DeployedAt)
	if err != nil { return nil, err }
	if gitURL.Valid { out.GitURL = gitURL.String }
	if commit.Valid { out.Commit = commit.String }
	if len(encrypted) > 0 {
		if len(encrypted) < 13 { return nil, shared.ErrInternal("stored Git credential is invalid") }
		plain, err := decryptCredential(s.credentialKey, encrypted[:12], encrypted[12:])
		if err != nil { return nil, shared.ErrInternal("could not decrypt Git credential") }
		out.GitToken = string(plain)
	}
	return &out, nil
}

func (s *Server) handleSiteGit(w http.ResponseWriter, r *http.Request, user *shared.User, deviceID int64, siteID string) {
	var owned int
	err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM sites st
		JOIN devices d ON d.id=st.device_id WHERE st.device_id=? AND st.site_id=? AND d.user_id=?`,
		deviceID, siteID, user.ID).Scan(&owned)
	if err != nil || owned != 1 { apiError(w, shared.ErrNotFound("site not found")); return }

	switch r.Method {
	case http.MethodGet:
		cfg, err := s.siteGitSettings(r.Context(), deviceID, siteID)
		if err != nil { apiError(w, shared.ErrInternal("could not load Git settings")); return }
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok": true, "repository_url": cfg.GitURL, "branch": cfg.GitBranch,
			"build_command": cfg.BuildCommand, "root_directory": cfg.RootDir, "health_check_path": cfg.HealthCheckPath,
			"clone_timeout_seconds": cfg.CloneTimeoutSeconds, "build_timeout_seconds": cfg.BuildTimeoutSeconds, "health_check_timeout_seconds": cfg.HealthCheckTimeoutSeconds, "start_command": cfg.StartCommand,
			"has_token": len(cfg.GitToken)>0, "commit": cfg.Commit,
			"deployed_at": func() interface{} { if cfg.DeployedAt.Valid { return cfg.DeployedAt.Time }; return nil }(),
		})
	case http.MethodPost:
		var in siteGitInput
		if err := decodeJSON(r, &in); err != nil { apiError(w, shared.ErrBadRequest("invalid JSON")); return }
		in.RepositoryURL = strings.TrimSpace(in.RepositoryURL)
		in.Branch = strings.TrimSpace(in.Branch)
		if in.Branch == "" { in.Branch = "main" }
		if in.RepositoryURL != "" && !validateGitRepository(in.RepositoryURL) {
			apiError(w, shared.ErrBadRequest("repository URL must be a valid HTTPS or SSH Git URL")); return
		}
		if !gitBranchPattern.MatchString(in.Branch) || strings.Contains(in.Branch, "..") || strings.Contains(in.Branch, "@{") || strings.HasSuffix(in.Branch, "/") || strings.HasSuffix(in.Branch, ".") {
			apiError(w, shared.ErrBadRequest("invalid Git branch")); return
		}
		if len(in.BuildCommand) > 512 || strings.ContainsAny(in.BuildCommand, "\x00\r\n") {
			apiError(w, shared.ErrBadRequest("build command is too long or contains invalid characters")); return
		}
		rootDir, ok := validateSiteRootDir(in.RootDirectory)
		if !ok { apiError(w, shared.ErrBadRequest("root directory must be inside the site directory")); return }
		startCommand := strings.TrimSpace(in.StartCommand)
		if len(startCommand) > 512 || strings.ContainsAny(startCommand, ";|&`$><\r\n\x00") { apiError(w, shared.ErrBadRequest("start command is too long or contains control characters")); return }
		if in.CloneTimeoutSeconds == 0 { in.CloneTimeoutSeconds = 600 }
		if in.CloneTimeoutSeconds < 30 || in.CloneTimeoutSeconds > 1800 { apiError(w, shared.ErrBadRequest("clone timeout must be between 30 and 1800 seconds")); return }
		if in.BuildTimeoutSeconds == 0 { in.BuildTimeoutSeconds = 1200 }
		if in.BuildTimeoutSeconds < 60 || in.BuildTimeoutSeconds > 7200 { apiError(w, shared.ErrBadRequest("build timeout must be between 60 and 7200 seconds")); return }
		if in.HealthCheckTimeoutSeconds == 0 { in.HealthCheckTimeoutSeconds = 15 }
		if in.HealthCheckTimeoutSeconds < 3 || in.HealthCheckTimeoutSeconds > 300 { apiError(w, shared.ErrBadRequest("health check timeout must be between 3 and 300 seconds")); return }
		healthPath := strings.TrimSpace(in.HealthCheckPath)
		if healthPath != "" && (len(healthPath) > 255 || !strings.HasPrefix(healthPath, "/") || strings.HasPrefix(healthPath, "//") || strings.ContainsAny(healthPath, "\r\n\x00") || strings.Contains(healthPath, "..")) {
			apiError(w, shared.ErrBadRequest("health check path must be a safe local URL path")); return
		}
		var encrypted interface{}
		if in.GitHubToken != "" {
			if len(in.GitHubToken) > 512 { apiError(w, shared.ErrBadRequest("GitHub token is too long")); return }
			nonce, ciphertext, err := encryptCredential(s.credentialKey, []byte(in.GitHubToken))
			if err != nil { apiError(w, shared.ErrInternal("could not encrypt GitHub token")); return }
			encrypted = append(nonce, ciphertext...)
		}
		_, err = s.db.conn.ExecContext(r.Context(), `UPDATE sites SET git_url=?,git_branch=?,build_command=?,root_dir=?,health_check_path=?,clone_timeout_seconds=?,build_timeout_seconds=?,health_check_timeout_seconds=?,start_command=?,
			git_token_enc=COALESCE(?,git_token_enc) WHERE device_id=? AND site_id=?`,
			nullableString(in.RepositoryURL), in.Branch, strings.TrimSpace(in.BuildCommand), rootDir, healthPath, in.CloneTimeoutSeconds, in.BuildTimeoutSeconds, in.HealthCheckTimeoutSeconds, startCommand, encrypted, deviceID, siteID)
		if err != nil { apiError(w, shared.ErrInternal("could not save Git settings")); return }
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok":true,"message":"Git settings saved","has_token":in.GitHubToken!=""})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func parseSiteGitRouteID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 { return 0, shared.ErrBadRequest("invalid device id") }
	return id, nil
}
