package control

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
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
	Commit struct { SHA string `json:"sha"` } `json:"commit"`
}

func parseGitHubRepositoryName(value string) (string, string, string, error) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "https://github.com/") {
		value = strings.TrimPrefix(value, "https://github.com/")
	} else if strings.Contains(value, "://") {
		return "", "", "", fmt.Errorf("repository URL must use github.com")
	}
	value = strings.Trim(strings.TrimSuffix(value, ".git"), "/")
	parts := strings.Split(value, "/")
	if len(parts) != 2 || !githubRepositoryPart.MatchString(parts[0]) || !githubRepositoryPart.MatchString(parts[1]) || parts[0] == "." || parts[0] == ".." || parts[1] == "." || parts[1] == ".." {
		return "", "", "", fmt.Errorf("repository must use the owner/repository format")
	}
	return parts[0], parts[1], parts[0] + "/" + parts[1], nil
}

func (s *Server) githubAppTokenForIntegration(ctx context.Context, userID, integrationID int64, owner string) (string, error) {
	var account string
	var nonce, ciphertext []byte
	if err := s.db.conn.QueryRowContext(ctx, `SELECT account_name,token_nonce,token_ciphertext FROM integrations WHERE id=? AND user_id=? AND provider='github_app'`, integrationID, userID).Scan(&account, &nonce, &ciphertext); err != nil {
		return "", fmt.Errorf("GitHub App installation not found")
	}
	if !strings.EqualFold(account, owner) { return "", fmt.Errorf("GitHub App installation owner mismatch") }
	raw, err := decryptCredential(s.credentialKey, nonce, ciphertext)
	if err != nil { return "", fmt.Errorf("could not decrypt GitHub App credentials") }
	var credentials githubAppCredentials
	if err := json.Unmarshal(raw, &credentials); err != nil { return "", fmt.Errorf("stored GitHub App credentials are invalid") }
	token, _, err := githubAppInstallationToken(ctx, credentials)
	return token, err
}

func githubAPIGet(ctx context.Context, token, endpoint string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil { return err }
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "Yevqo")
	resp, err := integrationHTTPClient.Do(req)
	if err != nil { return err }
	return readProviderJSON(resp, target)
}

func (s *Server) fetchGitHubRepository(ctx context.Context, token, owner, name string) (githubRepositoryDetails, error) {
	var out githubRepositoryDetails
	endpoint := "https://api.github.com/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name)
	err := githubAPIGet(ctx, token, endpoint, &out)
	if err != nil { return out, err }
	if out.ID < 1 || out.FullName == "" || out.CloneURL == "" || out.DefaultBranch == "" { return out, fmt.Errorf("GitHub returned incomplete repository metadata") }
	return out, nil
}

func (s *Server) fetchGitHubBranch(ctx context.Context, token, owner, name, branch string) (githubBranchDetails, error) {
	var out githubBranchDetails
	endpoint := "https://api.github.com/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name) + "/branches/" + url.PathEscape(branch)
	err := githubAPIGet(ctx, token, endpoint, &out)
	if err != nil { return out, err }
	if out.Name == "" || !validWebhookCommit(out.Commit.SHA) { return out, fmt.Errorf("GitHub returned invalid branch metadata") }
	return out, nil
}

func (s *Server) handleProjectRepository(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	raw := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/projects/"), "/")
	parts := strings.Split(raw, "/")
	if len(parts) < 2 || parts[1] != "repository" || len(parts) > 3 { http.NotFound(w, r); return }
	projectID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || projectID < 1 { apiError(w, shared.ErrBadRequest("invalid project id")); return }
	var ownerID int64
	if err := s.db.conn.QueryRowContext(r.Context(), "SELECT user_id FROM projects WHERE id=?", projectID).Scan(&ownerID); err != nil {
		apiError(w, shared.ErrNotFound("project not found")); return
	}
	action := "project.manage"
	if r.Method == http.MethodGet { action = "project.read" }
	if err := s.authorizeWorkspace(r.Context(), ownerID, u.ID, action); err != nil { apiError(w, err); return }

	sub := ""
	if len(parts) == 3 { sub = parts[2] }
	if sub != "" {
		switch sub {
		case "branches": s.handleProjectRepositoryBranches(w, r, ownerID, projectID); return
		case "pulls": s.handleProjectRepositoryPulls(w, r, ownerID, projectID); return
		case "checks": s.handleProjectRepositoryChecks(w, r, ownerID, projectID); return
		case "sites": s.handleProjectRepositorySites(w, r, u.ID, projectID, ownerID); return
		default: http.NotFound(w, r); return
		}
	}

	switch r.Method {
	case http.MethodGet:
		var integrationID, repositoryID int64
		var fullName, cloneURL, webURL, defaultBranch, deployBranch, language, lastCommit string
		var isPrivate, autoDeploy bool
		var createdAt, updatedAt time.Time
		err := s.db.conn.QueryRowContext(r.Context(), `SELECT integration_id,repository_id,full_name,clone_url,web_url,default_branch,deploy_branch,is_private,language,last_commit_sha,auto_deploy_enabled,created_at,updated_at FROM project_repositories WHERE project_id=?`, projectID).Scan(&integrationID,&repositoryID,&fullName,&cloneURL,&webURL,&defaultBranch,&deployBranch,&isPrivate,&language,&lastCommit,&autoDeploy,&createdAt,&updatedAt)
		if errors.Is(err, sql.ErrNoRows) { writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"repository":nil}); return }
		if err != nil { apiError(w, shared.ErrInternal("could not load project repository")); return }
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"repository":map[string]interface{}{"integration_id":integrationID,"repository_id":repositoryID,"full_name":fullName,"clone_url":cloneURL,"web_url":webURL,"default_branch":defaultBranch,"branch":deployBranch,"private":isPrivate,"language":language,"last_commit_sha":lastCommit,"auto_deploy_enabled":autoDeploy,"created_at":createdAt,"updated_at":updatedAt}})
	case http.MethodPost, http.MethodPut:
		var in projectRepositoryInput
		if err := decodeJSON(r,&in); err != nil { apiError(w,shared.ErrBadRequest("invalid JSON")); return }
		if in.IntegrationID < 1 { apiError(w,shared.ErrBadRequest("GitHub App integration is required")); return }
		owner, name, fullName, err := parseGitHubRepositoryName(in.FullName)
		if err != nil { apiError(w,shared.ErrBadRequest(err.Error())); return }
		ctx,cancel := context.WithTimeout(r.Context(),20*time.Second); defer cancel()
		token, err := s.githubAppTokenForIntegration(ctx,u.ID,in.IntegrationID,owner)
		if err != nil { apiError(w,shared.ErrForbidden("GitHub App installation cannot access this repository")); return }
		repository, err := s.fetchGitHubRepository(ctx,token,owner,name)
		if err != nil { apiError(w,shared.ErrBadRequest("GitHub repository could not be verified with this installation")); return }
		branch := strings.TrimSpace(in.Branch); if branch == "" { branch = repository.DefaultBranch }
		if !gitBranchPattern.MatchString(branch) || strings.Contains(branch,"..") || strings.Contains(branch,"@{") || strings.HasSuffix(branch,"/") || strings.HasSuffix(branch,".") {
			apiError(w,shared.ErrBadRequest("invalid deployment branch")); return
		}
		branchDetails, err := s.fetchGitHubBranch(ctx,token,owner,name,branch)
		if err != nil { apiError(w,shared.ErrBadRequest("selected branch is not available to this GitHub App installation")); return }
		_, err = s.db.conn.ExecContext(r.Context(), `INSERT INTO project_repositories(project_id,integration_id,repository_id,full_name,clone_url,web_url,default_branch,deploy_branch,is_private,language,last_commit_sha) VALUES(?,?,?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE integration_id=VALUES(integration_id),repository_id=VALUES(repository_id),full_name=VALUES(full_name),clone_url=VALUES(clone_url),web_url=VALUES(web_url),default_branch=VALUES(default_branch),deploy_branch=VALUES(deploy_branch),is_private=VALUES(is_private),language=VALUES(language),last_commit_sha=VALUES(last_commit_sha)`, projectID,in.IntegrationID,repository.ID,repository.FullName,repository.CloneURL,repository.HTMLURL,repository.DefaultBranch,branch,repository.Private,repository.Language,branchDetails.Commit.SHA)
		if err != nil { apiError(w,shared.ErrInternal("could not save project repository")); return }
		_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),WorkspaceOwnerID:shared.PtrInt64(ownerID),Action:"project_repository_connected",Details:map[string]interface{}{"project_id":projectID,"repository":repository.FullName,"branch":branch},IP:clientIP(r)})
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"repository":map[string]interface{}{"integration_id":in.IntegrationID,"repository_id":repository.ID,"full_name":repository.FullName,"clone_url":repository.CloneURL,"web_url":repository.HTMLURL,"default_branch":repository.DefaultBranch,"branch":branch,"private":repository.Private,"language":repository.Language,"last_commit_sha":branchDetails.Commit.SHA}})
	case http.MethodDelete:
		tx,err:=s.db.conn.BeginTx(r.Context(),nil);if err!=nil{apiError(w,shared.ErrInternal("could not start repository disconnect"));return}
		rows,err:=tx.QueryContext(r.Context(),`SELECT device_id,site_id,previous_git_url,previous_git_branch,previous_webhook_enabled,previous_webhook_secret_enc FROM project_repository_sites WHERE project_id=?`,projectID)
		if err!=nil{_=tx.Rollback();apiError(w,shared.ErrInternal("could not load linked deployment targets"));return}
		type restoreTarget struct{deviceID int64;siteID string;gitURL sql.NullString;branch string;webhook bool;secret []byte}
		targets:=make([]restoreTarget,0)
		for rows.Next(){var item restoreTarget;if err:=rows.Scan(&item.deviceID,&item.siteID,&item.gitURL,&item.branch,&item.webhook,&item.secret);err!=nil{rows.Close();_=tx.Rollback();apiError(w,shared.ErrInternal("could not read linked deployment targets"));return};targets=append(targets,item)}
		if err:=rows.Err();err!=nil{rows.Close();_=tx.Rollback();apiError(w,shared.ErrInternal("could not read linked deployment targets"));return};rows.Close()
		for _,item:=range targets{if _,err=tx.ExecContext(r.Context(),`UPDATE sites SET git_url=?,git_branch=?,github_webhook_enabled=?,github_webhook_secret_enc=?,github_webhook_updated_at=NOW() WHERE device_id=? AND site_id=?`,nullableString(item.gitURL.String),item.branch,item.webhook,item.secret,item.deviceID,item.siteID);err!=nil{_=tx.Rollback();apiError(w,shared.ErrInternal("could not restore linked site Git settings"));return}}
		if _,err=tx.ExecContext(r.Context(),"DELETE FROM project_repositories WHERE project_id=?",projectID);err!=nil{_=tx.Rollback();apiError(w,shared.ErrInternal("could not disconnect project repository"));return}
		if err=tx.Commit();err!=nil{apiError(w,shared.ErrInternal("could not commit repository disconnect"));return}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true})
	default: http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
	}
}

func (s *Server) projectRepositoryToken(ctx context.Context,userID,projectID int64)(string,string,string,string,error){
	var integrationID int64
	var fullName,cloneURL,branch,account string;var ownerID int64
	if err:=s.db.conn.QueryRowContext(ctx,`SELECT pr.integration_id,pr.full_name,pr.clone_url,pr.deploy_branch,i.account_name,p.user_id FROM project_repositories pr JOIN projects p ON p.id=pr.project_id JOIN integrations i ON i.id=pr.integration_id WHERE pr.project_id=? AND p.user_id=? AND i.user_id=p.user_id AND i.provider='github_app'`,projectID,userID).Scan(&integrationID,&fullName,&cloneURL,&branch,&account,&ownerID);err!=nil{return "","","","",err}
	owner,_,_,err:=parseGitHubRepositoryName(fullName);if err!=nil{return "","","","",err}
	token,err:=s.githubAppTokenForIntegration(ctx,ownerID,integrationID,owner)
	return token,fullName,cloneURL,branch,err
}

func (s *Server) handleProjectRepositoryBranches(w http.ResponseWriter,r *http.Request,userID,projectID int64){
	if r.Method!=http.MethodGet{http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return}
	ctx,cancel:=context.WithTimeout(r.Context(),60*time.Second);defer cancel()
	token,fullName,_,_,err:=s.projectRepositoryToken(ctx,userID,projectID);if err!=nil{apiError(w,shared.ErrNotFound("project repository not connected"));return}
	owner,name,_,_:=parseGitHubRepositoryName(fullName)
	requestedPage:=0;if raw:=r.URL.Query().Get("page");raw!=""{if n,e:=strconv.Atoi(raw);e==nil&&n>0&&n<=100{requestedPage=n}}
	type branchItem struct{Name string `json:"name"`;Protected bool `json:"protected"`;Commit struct{SHA string `json:"sha"`} `json:"commit"`}
	all:=make([]branchItem,0);startPage,endPage:=1,10;if requestedPage>0{startPage,endPage=requestedPage,requestedPage}
	nextPage:=0
	for page:=startPage;page<=endPage;page++{
		var items []branchItem
		endpoint:=fmt.Sprintf("https://api.github.com/repos/%s/%s/branches?per_page=100&page=%d",url.PathEscape(owner),url.PathEscape(name),page)
		if err:=githubAPIGet(ctx,token,endpoint,&items);err!=nil{writeJSON(w,http.StatusBadGateway,map[string]interface{}{"ok":false,"error":"GitHub branches are unavailable; verify Contents read permission"});return}
		all=append(all,items...)
		if len(items)<100{break}
		if page==endPage{nextPage=page+1}
	}
	out:=make([]map[string]interface{},0,len(all));for _,b:=range all{out=append(out,map[string]interface{}{"name":b.Name,"protected":b.Protected,"commit_sha":b.Commit.SHA})}
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"branches":out,"next_page":nextPage,"total_loaded":len(out)})
}

func (s *Server) handleProjectRepositoryPulls(w http.ResponseWriter,r *http.Request,userID,projectID int64){
	if r.Method!=http.MethodGet{http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return}
	ctx,cancel:=context.WithTimeout(r.Context(),60*time.Second);defer cancel()
	token,fullName,_,_,err:=s.projectRepositoryToken(ctx,userID,projectID);if err!=nil{apiError(w,shared.ErrNotFound("project repository not connected"));return}
	owner,name,_,_:=parseGitHubRepositoryName(fullName)
	state:=r.URL.Query().Get("state");if state!="closed"&&state!="all"{state="open"}
	type pullItem struct{Number int `json:"number"`;Title string `json:"title"`;State string `json:"state"`;HTMLURL string `json:"html_url"`;Draft bool `json:"draft"`;CreatedAt time.Time `json:"created_at"`;UpdatedAt time.Time `json:"updated_at"`;Head struct{Ref string `json:"ref"`;SHA string `json:"sha"`} `json:"head"`;Base struct{Ref string `json:"ref"`} `json:"base"`;User struct{Login string `json:"login"`} `json:"user"`}
	requestedPage:=0;if raw:=r.URL.Query().Get("page");raw!=""{if n,e:=strconv.Atoi(raw);e==nil&&n>0&&n<=100{requestedPage=n}}
	startPage,endPage:=1,10;if requestedPage>0{startPage,endPage=requestedPage,requestedPage}
	all:=make([]pullItem,0);nextPage:=0
	for page:=startPage;page<=endPage;page++{
		var items []pullItem
		endpoint:=fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?state=%s&per_page=100&page=%d",url.PathEscape(owner),url.PathEscape(name),url.QueryEscape(state),page)
		if err:=githubAPIGet(ctx,token,endpoint,&items);err!=nil{writeJSON(w,http.StatusBadGateway,map[string]interface{}{"ok":false,"error":"GitHub pull requests are unavailable; enable Pull requests read permission for the App"});return}
		all=append(all,items...);if len(items)<100{break};if page==endPage{nextPage=page+1}
	}
	out:=make([]map[string]interface{},0,len(all));for _,p:=range all{out=append(out,map[string]interface{}{"number":p.Number,"title":p.Title,"state":p.State,"url":p.HTMLURL,"draft":p.Draft,"created_at":p.CreatedAt,"updated_at":p.UpdatedAt,"head_branch":p.Head.Ref,"head_sha":p.Head.SHA,"base_branch":p.Base.Ref,"author":p.User.Login})}
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"pull_requests":out,"next_page":nextPage,"total_loaded":len(out)})
}

func (s *Server) handleProjectRepositoryChecks(w http.ResponseWriter,r *http.Request,userID,projectID int64){
	if r.Method!=http.MethodGet{http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return}
	ref:=strings.TrimSpace(r.URL.Query().Get("ref"));if ref==""||len(ref)>255||strings.ContainsAny(ref,"\r\n\x00"){apiError(w,shared.ErrBadRequest("valid branch or commit ref is required"));return}
	ctx,cancel:=context.WithTimeout(r.Context(),20*time.Second);defer cancel()
	token,fullName,_,_,err:=s.projectRepositoryToken(ctx,userID,projectID);if err!=nil{apiError(w,shared.ErrNotFound("project repository not connected"));return}
	owner,name,_,_:=parseGitHubRepositoryName(fullName)
	var checks struct{Total int `json:"total_count"`;Runs []struct{Name string `json:"name"`;Status string `json:"status"`;Conclusion string `json:"conclusion"`;HTMLURL string `json:"html_url"`;StartedAt *time.Time `json:"started_at"`;CompletedAt *time.Time `json:"completed_at"`} `json:"check_runs"`}
	checkURL:=fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s/check-runs?per_page=100",url.PathEscape(owner),url.PathEscape(name),url.PathEscape(ref))
	if err:=githubAPIGet(ctx,token,checkURL,&checks);err!=nil{writeJSON(w,http.StatusBadGateway,map[string]interface{}{"ok":false,"error":"GitHub checks are unavailable; enable Checks read permission for the App"});return}
	var status struct{State string `json:"state"`;Total int `json:"total_count"`;Statuses []struct{Context string `json:"context"`;State string `json:"state"`;Description string `json:"description"`;TargetURL string `json:"target_url"`} `json:"statuses"`}
	statusURL:=fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s/status",url.PathEscape(owner),url.PathEscape(name),url.PathEscape(ref))
	_ = githubAPIGet(ctx,token,statusURL,&status)
	runs:=make([]map[string]interface{},0,len(checks.Runs));for _,c:=range checks.Runs{runs=append(runs,map[string]interface{}{"name":c.Name,"status":c.Status,"conclusion":c.Conclusion,"url":c.HTMLURL,"started_at":c.StartedAt,"completed_at":c.CompletedAt})}
	statuses:=make([]map[string]interface{},0,len(status.Statuses));for _,st:=range status.Statuses{statuses=append(statuses,map[string]interface{}{"context":st.Context,"state":st.State,"description":st.Description,"url":st.TargetURL})}
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"ref":ref,"check_runs":runs,"check_total":checks.Total,"commit_status":status.State,"statuses":statuses})
}

func (s *Server) handleProjectRepositorySites(w http.ResponseWriter,r *http.Request,userID,projectID,ownerID int64){
	switch r.Method{
	case http.MethodGet:
		rows,err:=s.db.conn.QueryContext(r.Context(),`SELECT prs.device_id,prs.site_id,prs.auto_deploy_enabled,COALESCE(d.hostname,''),COALESCE(st.github_webhook_enabled,FALSE),prs.deploy_branch FROM project_repository_sites prs JOIN devices d ON d.id=prs.device_id JOIN sites st ON st.device_id=prs.device_id AND st.site_id=prs.site_id WHERE prs.project_id=? ORDER BY d.hostname,prs.site_id`,projectID)
		if err!=nil{apiError(w,shared.ErrInternal("could not load repository deployment targets"));return};defer rows.Close()
		out:=make([]map[string]interface{},0);for rows.Next(){var did int64;var sid,host,branch string;var auto,webhook bool;if err:=rows.Scan(&did,&sid,&auto,&host,&webhook,&branch);err!=nil{apiError(w,shared.ErrInternal("could not read repository deployment targets"));return};out=append(out,map[string]interface{}{"device_id":did,"site_id":sid,"device_name":host,"branch":branch,"auto_deploy_enabled":auto,"webhook_enabled":webhook})}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"targets":out})
	case http.MethodPost:
		var in struct{DeviceID int64 `json:"device_id"`;SiteID string `json:"site_id"`;Branch string `json:"branch"`;AutoDeploy bool `json:"auto_deploy_enabled"`;AllowProjectCode bool `json:"allow_project_code"`}
		if err:=decodeJSON(r,&in);err!=nil||in.DeviceID<1||!validLogSiteID(in.SiteID){apiError(w,shared.ErrBadRequest("valid device_id and site_id are required"));return}
		var fullName,cloneURL,branch string
		if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT full_name,clone_url,deploy_branch FROM project_repositories WHERE project_id=?`,projectID).Scan(&fullName,&cloneURL,&branch);err!=nil{apiError(w,shared.ErrBadRequest("connect a GitHub repository first"));return}
		var linked int
		if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM project_sites ps JOIN devices d ON d.id=ps.device_id JOIN sites st ON st.device_id=ps.device_id AND st.site_id=ps.site_id WHERE ps.project_id=? AND ps.device_id=? AND ps.site_id=? AND d.user_id=?`,projectID,in.DeviceID,in.SiteID,ownerID).Scan(&linked);err!=nil||linked!=1{apiError(w,shared.ErrBadRequest("site must already be attached to this project"));return}
		if strings.TrimSpace(in.Branch)!="" { branch=strings.TrimSpace(in.Branch) }; if !gitBranchPattern.MatchString(branch)||strings.Contains(branch,"..")||strings.Contains(branch,"@{")||strings.HasSuffix(branch,"/")||strings.HasSuffix(branch,"."){apiError(w,shared.ErrBadRequest("invalid deployment branch"));return}; if in.AutoDeploy&&!in.AllowProjectCode{apiError(w,shared.ErrForbidden("explicit consent to automatically execute repository code is required"));return}
		ctx,cancel:=context.WithTimeout(r.Context(),20*time.Second);defer cancel()
		token,repoName,_,_,tokenErr:=s.projectRepositoryToken(ctx,ownerID,projectID);if tokenErr!=nil{apiError(w,shared.ErrForbidden("GitHub App installation is unavailable"));return}
		owner,name,_,parseErr:=parseGitHubRepositoryName(repoName);if parseErr!=nil{apiError(w,shared.ErrBadRequest("invalid connected repository"));return}
		if _,branchErr:=s.fetchGitHubBranch(ctx,token,owner,name,branch);branchErr!=nil{apiError(w,shared.ErrBadRequest("deployment branch is not available in the repository"));return}
		var previousURL sql.NullString;var previousBranch string;var previousWebhookEnabled bool;var previousWebhookSecret []byte;var existingHookID int64
		previousErr:=s.db.conn.QueryRowContext(r.Context(),`SELECT previous_git_url,previous_git_branch,previous_webhook_enabled,previous_webhook_secret_enc,COALESCE(github_webhook_id,0) FROM project_repository_sites WHERE project_id=? AND device_id=? AND site_id=?`,projectID,in.DeviceID,in.SiteID).Scan(&previousURL,&previousBranch,&previousWebhookEnabled,&previousWebhookSecret,&existingHookID)
		var currentWebhookSecret []byte
		if errors.Is(previousErr,sql.ErrNoRows){
			if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT git_url,git_branch,github_webhook_enabled,github_webhook_secret_enc FROM sites WHERE device_id=? AND site_id=?`,in.DeviceID,in.SiteID).Scan(&previousURL,&previousBranch,&previousWebhookEnabled,&previousWebhookSecret);err!=nil{apiError(w,shared.ErrNotFound("site not found"));return}
		}else if previousErr!=nil{apiError(w,shared.ErrInternal("could not read original site Git settings"));return}
		_ = s.db.conn.QueryRowContext(r.Context(),`SELECT github_webhook_secret_enc FROM sites WHERE device_id=? AND site_id=?`,in.DeviceID,in.SiteID).Scan(&currentWebhookSecret)
		var secret string;var webhookSecret interface{}=previousWebhookSecret;webhookEnabled:=previousWebhookEnabled;hookID:=int64(0);hookRegistered:=false
		if in.AutoDeploy{
			if callbackURL,configured:=githubWebhookURLForSite(in.DeviceID,in.SiteID);configured{
				raw:=make([]byte,32);if _,err:=rand.Read(raw);err!=nil{apiError(w,shared.ErrInternal("could not create webhook secret"));return}
				secret=base64.RawURLEncoding.EncodeToString(raw);nonce,ciphertext,err:=encryptCredential(s.credentialKey,[]byte(secret));if err!=nil{apiError(w,shared.ErrInternal("could not encrypt webhook secret"));return};webhookSecret=append(nonce,ciphertext...);webhookEnabled=true
				registeredID,registerErr:=s.registerGitHubPushWebhook(ctx,token,owner,name,callbackURL,secret,existingHookID)
				if registerErr==nil{hookID=registeredID;hookRegistered=true}else if existingHookID>0{apiError(w,shared.ErrBadRequest("existing GitHub webhook could not be updated; grant repository administration write permission or remove the old webhook"));return}
			}
			if !hookRegistered{
				if existingHookID>0&&len(currentWebhookSecret)>0{webhookSecret=currentWebhookSecret;webhookEnabled=true;hookID=existingHookID;hookRegistered=true}else{
					raw:=make([]byte,32);if _,err:=rand.Read(raw);err!=nil{apiError(w,shared.ErrInternal("could not create webhook secret"));return}
					secret=base64.RawURLEncoding.EncodeToString(raw);nonce,ciphertext,err:=encryptCredential(s.credentialKey,[]byte(secret));if err!=nil{apiError(w,shared.ErrInternal("could not encrypt webhook secret"));return};webhookSecret=append(nonce,ciphertext...);webhookEnabled=true
				}
			}
		}
		tx,err:=s.db.conn.BeginTx(r.Context(),nil);if err!=nil{apiError(w,shared.ErrInternal("could not start repository target transaction"));return}
		if _,err=tx.ExecContext(r.Context(),`UPDATE sites SET git_url=?,git_branch=?,github_webhook_secret_enc=?,github_webhook_enabled=?,github_webhook_updated_at=NOW() WHERE device_id=? AND site_id=?`,cloneURL,branch,webhookSecret,webhookEnabled,in.DeviceID,in.SiteID);err==nil{
			_,err=tx.ExecContext(r.Context(),`INSERT INTO project_repository_sites(project_id,device_id,site_id,auto_deploy_enabled,deploy_branch,previous_git_url,previous_git_branch,previous_webhook_enabled,previous_webhook_secret_enc,github_webhook_id) VALUES(?,?,?,?,?,?,?,?,?,NULLIF(?,0)) ON DUPLICATE KEY UPDATE auto_deploy_enabled=VALUES(auto_deploy_enabled),deploy_branch=VALUES(deploy_branch),github_webhook_id=VALUES(github_webhook_id)`,projectID,in.DeviceID,in.SiteID,in.AutoDeploy,branch,nullableString(previousURL.String),previousBranch,previousWebhookEnabled,previousWebhookSecret,hookID)
		}
		if err!=nil{_=tx.Rollback();apiError(w,shared.ErrInternal("could not save repository deployment target"));return}
		if err=tx.Commit();err!=nil{apiError(w,shared.ErrInternal("could not commit repository deployment target"));return}
		_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(userID),WorkspaceOwnerID:shared.PtrInt64(ownerID),DeviceID:shared.PtrInt64(in.DeviceID),Action:"project_repository_site_connected",Details:map[string]interface{}{"project_id":projectID,"site_id":in.SiteID,"repository":fullName,"auto_deploy":in.AutoDeploy},IP:clientIP(r)})
		if !in.AutoDeploy&&existingHookID>0{_ = s.deleteGitHubPushWebhook(ctx,token,owner,name,existingHookID)}
		resp:=map[string]interface{}{"ok":true,"device_id":in.DeviceID,"site_id":in.SiteID,"branch":branch,"auto_deploy_enabled":in.AutoDeploy,"webhook_path":fmt.Sprintf("/webhooks/github/%d/%s",in.DeviceID,in.SiteID),"webhook_registered":hookRegistered}
		if secret!=""&&!hookRegistered{resp["webhook_secret"]=secret;resp["webhook_secret_once"]=true}
		writeJSON(w,http.StatusOK,resp)
	case http.MethodDelete:
		var in struct{DeviceID int64 `json:"device_id"`;SiteID string `json:"site_id"`}
		if err:=decodeJSON(r,&in);err!=nil||in.DeviceID<1||!validLogSiteID(in.SiteID){apiError(w,shared.ErrBadRequest("valid device_id and site_id are required"));return}
		var previousURL sql.NullString;var previousBranch string;var previousWebhookEnabled bool;var previousWebhookSecret []byte
		if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT previous_git_url,previous_git_branch,previous_webhook_enabled,previous_webhook_secret_enc FROM project_repository_sites WHERE project_id=? AND device_id=? AND site_id=?`,projectID,in.DeviceID,in.SiteID).Scan(&previousURL,&previousBranch,&previousWebhookEnabled,&previousWebhookSecret);err!=nil{apiError(w,shared.ErrNotFound("deployment target not found"));return}
		tx,err:=s.db.conn.BeginTx(r.Context(),nil);if err!=nil{apiError(w,shared.ErrInternal("could not start deployment target detach"));return}
		if _,err=tx.ExecContext(r.Context(),`UPDATE sites SET git_url=?,git_branch=?,github_webhook_enabled=?,github_webhook_secret_enc=?,github_webhook_updated_at=NOW() WHERE device_id=? AND site_id=?`,nullableString(previousURL.String),previousBranch,previousWebhookEnabled,previousWebhookSecret,in.DeviceID,in.SiteID);err==nil{_,err=tx.ExecContext(r.Context(),`DELETE FROM project_repository_sites WHERE project_id=? AND device_id=? AND site_id=?`,projectID,in.DeviceID,in.SiteID)}
		if err!=nil{_=tx.Rollback();apiError(w,shared.ErrInternal("could not restore original site Git settings"));return};if err=tx.Commit();err!=nil{apiError(w,shared.ErrInternal("could not commit deployment target detach"));return}
		_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(userID),WorkspaceOwnerID:shared.PtrInt64(ownerID),DeviceID:shared.PtrInt64(in.DeviceID),Action:"project_repository_site_disconnected",Details:map[string]interface{}{"project_id":projectID,"site_id":in.SiteID},IP:clientIP(r)})
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true})
	default:http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
	}
}
