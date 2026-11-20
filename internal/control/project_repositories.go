package control

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"veltrix/internal/shared"
)

var githubRepositoryPart = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}$`)

type projectRepositoryInput struct {
	IntegrationID int64  `json:"integration_id"`
	FullName      string `json:"full_name"`
	Branch        string `json:"branch"`
}

type githubRepositoryDetails struct {
	ID            int64  `json:"id"`
	FullName      string `json:"full_name"`
	CloneURL      string `json:"clone_url"`
	HTMLURL       string `json:"html_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
	Language      string `json:"language"`
}

type githubBranchDetails struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

func parseGitHubRepositoryName(value string) (string, string, string, error) {
	value = strings.TrimSpace(strings.TrimSuffix(value, ".git"))
	value = strings.TrimPrefix(value, "https://github.com/")
	value = strings.Trim(value, "/")
	parts := strings.Split(value, "/")
	if len(parts) != 2 || !githubRepositoryPart.MatchString(parts[0]) || !githubRepositoryPart.MatchString(parts[1]) {
		return "", "", "", fmt.Errorf("repository must use the owner/repository format")
	}
	return parts[0], parts[1], parts[0] + "/" + parts[1], nil
}

func (s *Server) fetchGitHubRepository(ctx context.Context, token, owner, name string) (githubRepositoryDetails, error) {
	var out githubRepositoryDetails
	endpoint := "https://api.github.com/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil { return out, err }
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "Yevqo")
	resp, err := integrationHTTPClient.Do(req)
	if err != nil { return out, err }
	if err := readProviderJSON(resp, &out); err != nil { return out, err }
	if out.ID < 1 || out.FullName == "" || out.CloneURL == "" || out.DefaultBranch == "" {
		return out, fmt.Errorf("GitHub returned incomplete repository metadata")
	}
	return out, nil
}

func (s *Server) fetchGitHubBranch(ctx context.Context, token, owner, name, branch string) (githubBranchDetails, error) {
	var out githubBranchDetails
	endpoint := "https://api.github.com/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name) + "/branches/" + url.PathEscape(branch)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil { return out, err }
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "Yevqo")
	resp, err := integrationHTTPClient.Do(req)
	if err != nil { return out, err }
	if err := readProviderJSON(resp, &out); err != nil { return out, err }
	if out.Name == "" || !validWebhookCommit(out.Commit.SHA) {
		return out, fmt.Errorf("GitHub returned invalid branch metadata")
	}
	return out, nil
}

func (s *Server) handleProjectRepository(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	raw := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/projects/"), "/")
	parts := strings.Split(raw, "/")
	if len(parts) != 2 || parts[1] != "repository" {
		http.NotFound(w, r)
		return
	}
	projectID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || projectID < 1 {
		apiError(w, shared.ErrBadRequest("invalid project id"))
		return
	}
	var ownerID int64
	if err := s.db.conn.QueryRowContext(r.Context(), "SELECT user_id FROM projects WHERE id=?", projectID).Scan(&ownerID); err != nil {
		apiError(w, shared.ErrNotFound("project not found"))
		return
	}
	action := "project.manage"
	if r.Method == http.MethodGet { action = "project.read" }
	if err := s.authorizeWorkspace(r.Context(), ownerID, u.ID, action); err != nil {
		apiError(w, err)
		return
	}

	switch r.Method {
	case http.MethodGet:
		var integrationID, repositoryID int64
		var fullName, cloneURL, webURL, defaultBranch, deployBranch, language, lastCommit string
		var isPrivate, autoDeploy bool
		var createdAt, updatedAt time.Time
		err := s.db.conn.QueryRowContext(r.Context(), `SELECT integration_id,repository_id,full_name,clone_url,web_url,default_branch,deploy_branch,is_private,language,last_commit_sha,auto_deploy_enabled,created_at,updated_at FROM project_repositories WHERE project_id=?`, projectID).Scan(&integrationID,&repositoryID,&fullName,&cloneURL,&webURL,&defaultBranch,&deployBranch,&isPrivate,&language,&lastCommit,&autoDeploy,&createdAt,&updatedAt)
		if err != nil {
			if err.Error() == "sql: no rows in result set" { writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"repository":nil}); return }
			apiError(w, shared.ErrInternal("could not load project repository"))
			return
		}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"repository":map[string]interface{}{"integration_id":integrationID,"repository_id":repositoryID,"full_name":fullName,"clone_url":cloneURL,"web_url":webURL,"default_branch":defaultBranch,"branch":deployBranch,"private":isPrivate,"language":language,"last_commit_sha":lastCommit,"auto_deploy_enabled":autoDeploy,"created_at":createdAt,"updated_at":updatedAt}})
	case http.MethodPost, http.MethodPut:
		var in projectRepositoryInput
		if err := decodeJSON(r,&in); err != nil { apiError(w,shared.ErrBadRequest("invalid JSON")); return }
		if in.IntegrationID < 1 { apiError(w,shared.ErrBadRequest("GitHub App integration is required")); return }
		owner, name, fullName, err := parseGitHubRepositoryName(in.FullName)
		if err != nil { apiError(w,shared.ErrBadRequest(err.Error())); return }
		var accountName string
		if err := s.db.conn.QueryRowContext(r.Context(), `SELECT account_name FROM integrations WHERE id=? AND user_id=? AND provider='github_app'`, in.IntegrationID, u.ID).Scan(&accountName); err != nil {
			apiError(w, shared.ErrNotFound("GitHub App integration not found"))
			return
		}
		if !strings.EqualFold(accountName, owner) {
			apiError(w, shared.ErrForbidden("repository owner does not match the selected GitHub App installation"))
			return
		}
		token, err := s.githubAppTokenForRepository(r.Context(),u.ID,"https://github.com/"+fullName+".git")
		if err != nil { apiError(w,shared.ErrBadRequest("GitHub App installation could not access this repository")); return }
		ctx,cancel := context.WithTimeout(r.Context(),20*time.Second)
		defer cancel()
		repository, err := s.fetchGitHubRepository(ctx,token,owner,name)
		if err != nil { apiError(w,shared.ErrBadRequest("GitHub repository could not be verified with this installation")); return }
		branch := strings.TrimSpace(in.Branch)
		if branch == "" { branch = repository.DefaultBranch }
		if len(branch) > 255 || strings.ContainsAny(branch,"\r\n\x00") {
			apiError(w,shared.ErrBadRequest("invalid deployment branch"))
			return
		}
		branchDetails, err := s.fetchGitHubBranch(ctx,token,owner,name,branch)
		if err != nil { apiError(w,shared.ErrBadRequest("selected branch is not available to this GitHub App installation")); return }
		_, err = s.db.conn.ExecContext(r.Context(), `INSERT INTO project_repositories(project_id,integration_id,repository_id,full_name,clone_url,web_url,default_branch,deploy_branch,is_private,language,last_commit_sha) VALUES(?,?,?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE integration_id=VALUES(integration_id),repository_id=VALUES(repository_id),full_name=VALUES(full_name),clone_url=VALUES(clone_url),web_url=VALUES(web_url),default_branch=VALUES(default_branch),deploy_branch=VALUES(deploy_branch),is_private=VALUES(is_private),language=VALUES(language),last_commit_sha=VALUES(last_commit_sha)`, projectID,in.IntegrationID,repository.ID,repository.FullName,repository.CloneURL,repository.HTMLURL,repository.DefaultBranch,branch,repository.Private,repository.Language,branchDetails.Commit.SHA)
		if err != nil { apiError(w,shared.ErrInternal("could not save project repository")); return }
		_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),WorkspaceOwnerID:shared.PtrInt64(ownerID),Action:"project_repository_connected",Details:map[string]interface{}{"project_id":projectID,"repository":repository.FullName,"branch":branch},IP:clientIP(r)})
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"repository":map[string]interface{}{"repository_id":repository.ID,"full_name":repository.FullName,"clone_url":repository.CloneURL,"web_url":repository.HTMLURL,"default_branch":repository.DefaultBranch,"branch":branch,"private":repository.Private,"language":repository.Language,"last_commit_sha":branchDetails.Commit.SHA}})
	case http.MethodDelete:
		res, err := s.db.conn.ExecContext(r.Context(),"DELETE FROM project_repositories WHERE project_id=?",projectID)
		if err != nil { apiError(w,shared.ErrInternal("could not disconnect project repository")); return }
		n,_ := res.RowsAffected()
		if n == 0 { apiError(w,shared.ErrNotFound("project repository is not connected")); return }
		_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),WorkspaceOwnerID:shared.PtrInt64(ownerID),Action:"project_repository_disconnected",Details:map[string]interface{}{"project_id":projectID},IP:clientIP(r)})
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true})
	default:
		http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
	}
}
