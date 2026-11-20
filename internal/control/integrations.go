package control

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"veltrix/internal/shared"
)

type integrationConnectInput struct {
	Provider string `json:"provider"`
	Token string `json:"token"`
	AppID string `json:"app_id"`
	InstallationID string `json:"installation_id"`
	PrivateKey string `json:"private_key"`
}

type providerAccount struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Scopes  string `json:"scopes"`
}

type providerRepository struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
	WebURL        string `json:"web_url"`
}

var integrationHTTPClient = &http.Client{Timeout: 15 * time.Second}

func providerRequest(ctx context.Context, provider, token, endpoint string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil { return nil, err }
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "VELTRIX-Dock")
	if provider == "github" {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	} else {
		req.Header.Set("PRIVATE-TOKEN", token)
	}
	return integrationHTTPClient.Do(req)
}

func readProviderJSON(resp *http.Response, target interface{}) error {
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("provider returned HTTP %d", resp.StatusCode)
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 4<<20))
	return dec.Decode(target)
}

func verifyProviderToken(ctx context.Context, provider, token string) (providerAccount, string, error) {
	var account providerAccount
	var endpoint string
	switch provider {
	case "github":
		endpoint = "https://api.github.com/user"
	case "gitlab":
		endpoint = "https://gitlab.com/api/v4/user"
	default:
		return account, "", fmt.Errorf("unsupported provider")
	}
	resp, err := providerRequest(ctx, provider, token, endpoint)
	if err != nil { return account, "", err }
	if provider == "github" {
		var v struct { ID int64 `json:"id"`; Login string `json:"login"`; Name string `json:"name"`; Email string `json:"email"` }
		if err := readProviderJSON(resp, &v); err != nil { return account, "", err }
		account = providerAccount{ID: strconv.FormatInt(v.ID,10), Name: v.Login, Email: v.Email}
		if account.Name == "" { account.Name = v.Name }
	} else {
		var v struct { ID int64 `json:"id"`; Username string `json:"username"`; Name string `json:"name"`; Email string `json:"public_email"` }
		if err := readProviderJSON(resp, &v); err != nil { return account, "", err }
		account = providerAccount{ID: strconv.FormatInt(v.ID,10), Name: v.Username, Email: v.Email}
		if account.Name == "" { account.Name = v.Name }
	}
	if account.ID == "" || account.Name == "" { return account, "", fmt.Errorf("provider returned incomplete account details") }
	return account, "", nil
}

func (s *Server) handleIntegrations(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	switch r.Method {
	case http.MethodGet:
		rows, err := s.db.conn.QueryContext(r.Context(), `SELECT id,provider,account_id,account_name,account_email,scopes,connected_at,last_verified_at FROM integrations WHERE user_id=? ORDER BY connected_at DESC`, u.ID)
		if err != nil { apiError(w, shared.ErrInternal("could not load integrations")); return }
		defer rows.Close()
		items := make([]map[string]interface{},0)
		for rows.Next() {
			var id int64
			var provider, accountID, accountName, scopes string
			var email *string
			var connected time.Time
			var verified *time.Time
			if err := rows.Scan(&id,&provider,&accountID,&accountName,&email,&scopes,&connected,&verified); err != nil { apiError(w,shared.ErrInternal("could not read integrations")); return }
			items=append(items,map[string]interface{}{"id":id,"provider":provider,"account_id":accountID,"account_name":accountName,"account_email":email,"scopes":scopes,"connected_at":connected,"last_verified_at":verified,"status":"connected"})
		}
		if err:=rows.Err();err!=nil { apiError(w,shared.ErrInternal("could not read integrations"));return }
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"integrations":items})
	case http.MethodPost:
		var in integrationConnectInput
		if err:=decodeJSON(r,&in);err!=nil { apiError(w,shared.ErrBadRequest("invalid JSON"));return }
		in.Provider=strings.ToLower(strings.TrimSpace(in.Provider))
		in.Token=strings.TrimSpace(in.Token)
		if in.Provider == "github_app" {
			credentials := githubAppCredentials{AppID:strings.TrimSpace(in.AppID),InstallationID:strings.TrimSpace(in.InstallationID),PrivateKey:in.PrivateKey}
			if err := validateGitHubAppCredentials(credentials); err != nil { apiError(w,shared.ErrBadRequest(err.Error())); return }
			ctx,cancel := context.WithTimeout(r.Context(),20*time.Second); defer cancel()
			_, account, err := githubAppInstallationToken(ctx,credentials)
			if err != nil { apiError(w,shared.ErrBadRequest("GitHub App installation could not be verified; check App ID, installation ID and private key")); return }
			raw,err := json.Marshal(credentials); if err != nil { apiError(w,shared.ErrInternal("could not encode GitHub App credentials")); return }
			nonce,ciphertext,err := encryptCredential(s.credentialKey,raw); if err != nil { apiError(w,shared.ErrInternal("could not encrypt GitHub App credentials")); return }
			_,err = s.db.conn.ExecContext(r.Context(),`INSERT INTO integrations(user_id,provider,account_id,account_name,account_email,token_nonce,token_ciphertext,scopes,last_verified_at) VALUES(?,'github_app',?,?,NULL,?,?,?,?) ON DUPLICATE KEY UPDATE account_name=VALUES(account_name),token_nonce=VALUES(token_nonce),token_ciphertext=VALUES(token_ciphertext),scopes=VALUES(scopes),last_verified_at=VALUES(last_verified_at)`,u.ID,credentials.InstallationID,account,nonce,ciphertext,"contents:read, metadata:read",time.Now())
			if err != nil { apiError(w,shared.ErrInternal("could not save GitHub App integration")); return }
			_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),Action:"integration_connected",Details:map[string]interface{}{"provider":"github_app","account":account}})
			writeJSON(w,http.StatusCreated,map[string]interface{}{"ok":true,"provider":"github_app","account":account,"message":"GitHub App installation connected"})
			return
		}
		if in.Provider!="github" && in.Provider!="gitlab" { apiError(w,shared.ErrBadRequest("supported providers: github, github_app, gitlab"));return }
		if len(in.Token)<10 || len(in.Token)>4096 || strings.ContainsAny(in.Token,"\r\n\x00") { apiError(w,shared.ErrBadRequest("enter a valid access token"));return }
		ctx,cancel:=context.WithTimeout(r.Context(),20*time.Second);defer cancel()
		account,scopes,err:=verifyProviderToken(ctx,in.Provider,in.Token)
		if err!=nil { apiError(w,shared.ErrBadRequest("provider rejected this token or could not be reached"));return }
		nonce,ciphertext,err:=encryptCredential(s.credentialKey,[]byte(in.Token))
		if err!=nil { apiError(w,shared.ErrInternal("could not encrypt provider token"));return }
		_,err=s.db.conn.ExecContext(r.Context(),`INSERT INTO integrations(user_id,provider,account_id,account_name,account_email,token_nonce,token_ciphertext,scopes,last_verified_at) VALUES(?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE account_name=VALUES(account_name),account_email=VALUES(account_email),token_nonce=VALUES(token_nonce),token_ciphertext=VALUES(token_ciphertext),scopes=VALUES(scopes),last_verified_at=VALUES(last_verified_at)`,u.ID,in.Provider,account.ID,account.Name,nullableString(account.Email),nonce,ciphertext,scopes,time.Now())
		if err!=nil { apiError(w,shared.ErrInternal("could not save integration"));return }
		_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),Action:"integration_connected",Details:map[string]interface{}{"provider":in.Provider,"account":account.Name}})
		writeJSON(w,http.StatusCreated,map[string]interface{}{"ok":true,"provider":in.Provider,"account":account.Name,"message":"Integration connected"})
	default:
		http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleIntegrationRepositories(w http.ResponseWriter, r *http.Request) {
	u:=UserFromContext(r.Context())
	raw:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/integrations/"),"/")
	parts:=strings.Split(raw,"/")
	if len(parts)!=2 || parts[1]!="repositories" { http.NotFound(w,r);return }
	id,err:=strconv.ParseInt(parts[0],10,64)
	if err!=nil || id<1 { apiError(w,shared.ErrBadRequest("invalid integration id"));return }
	var provider string
	var nonce,ciphertext []byte
	err=s.db.conn.QueryRowContext(r.Context(),`SELECT provider,token_nonce,token_ciphertext FROM integrations WHERE id=? AND user_id=?`,id,u.ID).Scan(&provider,&nonce,&ciphertext)
	if err!=nil { apiError(w,shared.ErrNotFound("integration not found"));return }
	rawCredential,err:=decryptCredential(s.credentialKey,nonce,ciphertext)
	if err!=nil { apiError(w,shared.ErrInternal("could not decrypt provider credentials"));return }
	ctx,cancel:=context.WithTimeout(r.Context(),20*time.Second);defer cancel()
	token := string(rawCredential)
	isGitHubApp := provider=="github_app"
	if isGitHubApp {
		var credentials githubAppCredentials
		if err:=json.Unmarshal(rawCredential,&credentials);err!=nil { apiError(w,shared.ErrInternal("stored GitHub App credentials are invalid"));return }
		token,_,err=githubAppInstallationToken(ctx,credentials)
		if err!=nil { apiError(w,shared.ErrBadRequest("GitHub App installation token could not be refreshed"));return }
	}
	var repos []providerRepository
	if provider=="github" || isGitHubApp {
		endpoint := "https://api.github.com/user/repos?per_page=100&sort=updated"
		if isGitHubApp { endpoint = "https://api.github.com/installation/repositories?per_page=100" }
		resp,err:=providerRequest(ctx,"github",string(token),endpoint)
		if err!=nil { apiError(w,shared.ErrInternal("GitHub API is unavailable"));return }
		var values []struct { ID int64 `json:"id"`; Name string `json:"name"`; FullName string `json:"full_name"`; CloneURL string `json:"clone_url"`; DefaultBranch string `json:"default_branch"`; Private bool `json:"private"`; HTMLURL string `json:"html_url"` }
		if err:=readProviderJSON(resp,&values);err!=nil { apiError(w,shared.ErrBadRequest("GitHub rejected repository access"));return }
		for _,v:=range values { repos=append(repos,providerRepository{ID:strconv.FormatInt(v.ID,10),Name:v.Name,FullName:v.FullName,CloneURL:v.CloneURL,DefaultBranch:v.DefaultBranch,Private:v.Private,WebURL:v.HTMLURL}) }
	} else if provider=="gitlab" {
		resp,err:=providerRequest(ctx,provider,string(token),"https://gitlab.com/api/v4/projects?membership=true&per_page=100&order_by=last_activity_at")
		if err!=nil { apiError(w,shared.ErrInternal("GitLab API is unavailable"));return }
		var values []struct { ID int64 `json:"id"`; Name string `json:"name"`; PathWithNamespace string `json:"path_with_namespace"`; HTTPURL string `json:"http_url_to_repo"`; DefaultBranch string `json:"default_branch"`; Visibility string `json:"visibility"`; WebURL string `json:"web_url"` }
		if err:=readProviderJSON(resp,&values);err!=nil { apiError(w,shared.ErrBadRequest("GitLab rejected project access"));return }
		for _,v:=range values { repos=append(repos,providerRepository{ID:strconv.FormatInt(v.ID,10),Name:v.Name,FullName:v.PathWithNamespace,CloneURL:v.HTTPURL,DefaultBranch:v.DefaultBranch,Private:v.Visibility!="public",WebURL:v.WebURL}) }
	} else { apiError(w,shared.ErrBadRequest("unsupported provider"));return }
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"repositories":repos})
}

func (s *Server) handleIntegrationByID(w http.ResponseWriter, r *http.Request) {
	u:=UserFromContext(r.Context())
	if r.Method!=http.MethodDelete { http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return }
	raw:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/integrations/"),"/")
	id,err:=strconv.ParseInt(raw,10,64)
	if err!=nil||id<1 { apiError(w,shared.ErrBadRequest("invalid integration id"));return }
	var provider,account string
	err=s.db.conn.QueryRowContext(r.Context(),`SELECT provider,account_name FROM integrations WHERE id=? AND user_id=?`,id,u.ID).Scan(&provider,&account)
	if err!=nil { apiError(w,shared.ErrNotFound("integration not found"));return }
	if _,err=s.db.conn.ExecContext(r.Context(),`DELETE FROM integrations WHERE id=? AND user_id=?`,id,u.ID);err!=nil { apiError(w,shared.ErrInternal("could not revoke integration"));return }
	_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),Action:"integration_disconnected",Details:map[string]interface{}{"provider":provider,"account":account}})
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true})
}

func integrationProviderLabel(provider string) string {
	switch provider { case "github":return "GitHub";case "gitlab":return "GitLab";default:return provider }
}

