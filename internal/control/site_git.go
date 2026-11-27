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

// validateGitBranchName applies Git ref-format restrictions consistently before
// a branch is persisted or passed to GitHub/Agent operations.
func validateGitBranchName(branch string) bool {
	if !gitBranchPattern.MatchString(branch) || branch == "" || strings.Contains(branch, "..") || strings.Contains(branch, "@{") || strings.HasSuffix(branch, "/") || strings.HasSuffix(branch, ".") {
		return false
	}
	for _, component := range strings.Split(branch, "/") {
		if component == "" || strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".lock") {
			return false
		}
	}
	return true
}

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
		&gitURL, &out.GitBranch, &out.BuildCommand, &out.RootDir, &out.HealthCheckPath, &out.CloneTimeoutSeconds, &out.BuildTimeoutSeconds, &out.HealthCheckTimeoutSeconds, &out.StartCommand, &encrypted, &commit, &out.DeployedAt)
	if err != nil { return nil, err }
	if gitURL.Valid { out.GitURL = gitURL.String }
	if commit.Valid { out.Commit = commit.String }
	if len(encrypted) > 0 {
		if len(encrypted) < 13 { return nil, shared.ErrInternal("stored Git credential is invalid") }
		plain, err := decryptCredential(s.credentialKey, encrypted[:12], encrypted[12:])
		if err != nil { return nil, shared.ErrInternal("could not decrypt Git credential") }
		out.GitToken = string(plain)
	}
	if out.GitURL != "" {
		var integrationID,integrationUserID int64
		var fullName string
		mapErr:=s.db.conn.QueryRowContext(ctx,`SELECT pr.integration_id,pr.full_name,i.user_id FROM project_repository_sites prs JOIN project_repositories pr ON pr.project_id=prs.project_id JOIN integrations i ON i.id=pr.integration_id WHERE prs.device_id=? AND prs.site_id=? LIMIT 1`,deviceID,siteID).Scan(&integrationID,&fullName,&integrationUserID)
		if mapErr==nil {
			if _,_,_,parseErr:=parseGitHubRepositoryName(fullName);parseErr!=nil{return nil,shared.ErrInternal("linked GitHub repository metadata is invalid")}
			token,tokenErr:=s.githubAppTokenForIntegrationRepository(ctx,integrationUserID,integrationID,fullName);if tokenErr!=nil{return nil,shared.ErrInternal("could not refresh linked GitHub App installation token")};out.GitToken=token
		} else if mapErr==sql.ErrNoRows && out.GitToken=="" {
			var userID int64
			if err := s.db.conn.QueryRowContext(ctx,`SELECT user_id FROM devices WHERE id=?`,deviceID).Scan(&userID); err != nil { return nil, err }
			token, err := s.githubAppTokenForRepository(ctx,userID,out.GitURL)
			if err != nil { return nil, shared.ErrInternal("could not refresh GitHub App installation token") }
			out.GitToken = token
		} else if mapErr != sql.ErrNoRows { return nil, mapErr }
	}
	return &out, nil
}

func (s *Server) handleSiteGit(w http.ResponseWriter, r *http.Request, user *shared.User, deviceID int64, siteID string) {
	dev, err := s.db.GetDeviceByID(r.Context(), deviceID)
	if err != nil { apiError(w, shared.ErrNotFound("site not found")); return }
	projectID, scopeErr := s.siteProjectScope(r.Context(), dev.UserID, deviceID, siteID)
	if scopeErr != nil { apiError(w, scopeErr); return }
	if projectID > 0 {
		action := "project.read"
		if r.Method != http.MethodGet { action = "project.manage" }
		ownerID, authErr := s.authorizeProjectOwner(r.Context(), projectID, user.ID, action)
		if authErr != nil { apiError(w, authErr); return }
		if ownerID != dev.UserID { apiError(w, shared.ErrNotFound("site not found")); return }
		if authErr := s.authorizeSiteProjects(r.Context(), deviceID, siteID, dev.UserID, user.ID, action); authErr != nil { apiError(w, authErr); return }
	} else {
		if err := s.authorizeWorkspace(r.Context(), dev.UserID, user.ID, "device.read"); err != nil { apiError(w, err); return }
		if r.Method != http.MethodGet {
			if err := s.authorizeWorkspace(r.Context(), dev.UserID, user.ID, "site.configure"); err != nil { apiError(w, err); return }
		}
	}

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
		var mappedProjectID int64;var mappedRepositoryURL,mappedRepositoryName,mappedBranch,mappedCommitSHA string
		mapErr:=s.db.conn.QueryRowContext(r.Context(),`SELECT prs.project_id,pr.clone_url,pr.full_name,prs.deploy_branch FROM project_repository_sites prs JOIN project_repositories pr ON pr.project_id=prs.project_id WHERE prs.device_id=? AND prs.site_id=? LIMIT 1`,deviceID,siteID).Scan(&mappedProjectID,&mappedRepositoryURL,&mappedRepositoryName,&mappedBranch)
		if mapErr==nil {
			if err:=s.authorizeProjectAction(r.Context(),mappedProjectID,user.ID,"project.manage");err!=nil{apiError(w,err);return}
			if err:=s.requireProjectSite(r.Context(),mappedProjectID,deviceID,siteID);err!=nil{apiError(w,err);return}
			if in.RepositoryURL!=""{owner,name,_,parseErr:=parseGitHubRepositoryName(in.RepositoryURL);expectedOwner,expectedName,_,expectedErr:=parseGitHubRepositoryName(mappedRepositoryName);if parseErr!=nil||expectedErr!=nil||!strings.EqualFold(owner,expectedOwner)||!strings.EqualFold(name,expectedName){apiError(w,shared.ErrBadRequest("the Git repository is managed by this project's Source Repository settings"));return}}
			in.RepositoryURL=mappedRepositoryURL
			if in.Branch==""{in.Branch=mappedBranch}
			if in.GitHubToken!=""{apiError(w,shared.ErrBadRequest("project-linked repositories use the GitHub App installation; do not store a personal token on the site"));return}
			if in.Branch!=mappedBranch{token,fullName,_,_,tokenErr:=s.projectRepositoryToken(r.Context(),dev.UserID,mappedProjectID);if tokenErr!=nil{apiError(w,shared.ErrForbidden("project GitHub App installation is unavailable"));return};owner,name,_,_:=parseGitHubRepositoryName(fullName);branchInfo,branchErr:=s.fetchGitHubBranch(r.Context(),token,owner,name,in.Branch);if branchErr!=nil{apiError(w,shared.ErrBadRequest("selected branch is not available in the connected repository"));return};mappedCommitSHA=branchInfo.Commit.SHA}
		} else if mapErr!=sql.ErrNoRows { apiError(w,shared.ErrInternal("could not inspect project repository mapping"));return }
		if in.Branch == "" { in.Branch = "main" }
		if in.RepositoryURL != "" && !validateGitRepository(in.RepositoryURL) {
			apiError(w, shared.ErrBadRequest("repository URL must be a valid HTTPS or SSH Git URL")); return
		}
		if !validateGitBranchName(in.Branch) {
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
		tx,txErr:=s.db.conn.BeginTx(r.Context(),nil);if txErr!=nil{apiError(w,shared.ErrInternal("could not start Git settings update"));return}
		_, err = tx.ExecContext(r.Context(), `UPDATE sites SET git_url=?,git_branch=?,build_command=?,root_dir=?,health_check_path=?,clone_timeout_seconds=?,build_timeout_seconds=?,health_check_timeout_seconds=?,start_command=?,
			git_token_enc=COALESCE(?,git_token_enc) WHERE device_id=? AND site_id=?`,
			nullableString(in.RepositoryURL), in.Branch, strings.TrimSpace(in.BuildCommand), rootDir, healthPath, in.CloneTimeoutSeconds, in.BuildTimeoutSeconds, in.HealthCheckTimeoutSeconds, startCommand, encrypted, deviceID, siteID)
		if err==nil&&mappedProjectID>0&&in.Branch!=mappedBranch{_,err=tx.ExecContext(r.Context(),`UPDATE project_repository_sites SET deploy_branch=? WHERE project_id=? AND device_id=? AND site_id=?`,in.Branch,mappedProjectID,deviceID,siteID);if err==nil&&mappedCommitSHA!=""{_,err=tx.ExecContext(r.Context(),`UPDATE project_repositories SET last_commit_sha=? WHERE project_id=?`,mappedCommitSHA,mappedProjectID)}}
		if err!=nil{_=tx.Rollback();apiError(w,shared.ErrInternal("could not save Git settings"));return}
		if err=tx.Commit();err!=nil{apiError(w,shared.ErrInternal("could not commit Git settings"));return}
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
