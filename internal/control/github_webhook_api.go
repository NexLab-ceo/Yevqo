package control

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type githubWebhookRegistration struct {
	ID int64 `json:"id"`
}

func githubWebhookAPIRequest(ctx context.Context, token, method, endpoint string, payload interface{}, target interface{}) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil { return err }
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil { return err }
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "Yevqo")
	if payload != nil { req.Header.Set("Content-Type", "application/json") }
	resp, err := integrationHTTPClient.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("GitHub webhook API returned HTTP %d", resp.StatusCode)
	}
	if target != nil {
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(target); err != nil { return err }
	}
	return nil
}

func publicYevqoBaseURL() (string, bool) {
	raw := strings.TrimSpace(os.Getenv("YEVQO_PUBLIC_URL"))
	if raw == "" { return "", false }
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", false
	}
	return strings.TrimRight(parsed.String(), "/"), true
}

func githubWebhookURLForSite(deviceID int64, siteID string) (string, bool) {
	base, ok := publicYevqoBaseURL()
	if !ok { return "", false }
	return base + fmt.Sprintf("/webhooks/github/%s/%s", strconv.FormatInt(deviceID, 10), url.PathEscape(siteID)), true
}

func (s *Server) registerGitHubPushWebhook(ctx context.Context, token, owner, repository, callbackURL, secret string, existingID int64) (int64, error) {
	parsed, err := url.Parse(callbackURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return 0, fmt.Errorf("a valid HTTPS YEVQO_PUBLIC_URL is required")
	}
	config := map[string]string{"url": callbackURL, "content_type": "json", "secret": secret, "insecure_ssl": "0"}
	payload := map[string]interface{}{"name": "web", "active": true, "events": []string{"push"}, "config": config}
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/hooks", url.PathEscape(owner), url.PathEscape(repository))
	if existingID > 0 {
		endpoint += "/" + strconv.FormatInt(existingID, 10)
		var result githubWebhookRegistration
		if err := githubWebhookAPIRequest(ctx, token, http.MethodPatch, endpoint, payload, &result); err != nil { return 0, err }
		if result.ID > 0 { return result.ID, nil }
		return existingID, nil
	}
	var result githubWebhookRegistration
	if err := githubWebhookAPIRequest(ctx, token, http.MethodPost, endpoint, payload, &result); err != nil { return 0, err }
	if result.ID < 1 { return 0, fmt.Errorf("GitHub did not return a webhook ID") }
	return result.ID, nil
}

func (s *Server) deleteGitHubPushWebhook(ctx context.Context, token, owner, repository string, hookID int64) error {
	if hookID < 1 { return nil }
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/hooks/%d", url.PathEscape(owner), url.PathEscape(repository), hookID)
	return githubWebhookAPIRequest(ctx, token, http.MethodDelete, endpoint, nil, nil)
}

func (s *Server) deleteProjectRepositorySiteHook(ctx context.Context, ownerID, projectID, deviceID int64, siteID string) {
	var hookID int64
	var fullName string
	var integrationID int64
	err:=s.db.conn.QueryRowContext(ctx,`SELECT COALESCE(prs.github_webhook_id,0),pr.full_name,pr.integration_id FROM project_repository_sites prs JOIN project_repositories pr ON pr.project_id=prs.project_id WHERE prs.project_id=? AND prs.device_id=? AND prs.site_id=?`,projectID,deviceID,siteID).Scan(&hookID,&fullName,&integrationID)
	if err!=nil||hookID<1{return}
	owner,name,_,err:=parseGitHubRepositoryName(fullName);if err!=nil{return}
	token,err:=s.githubAppTokenForIntegration(ctx,ownerID,integrationID,owner);if err!=nil{return}
	_ = s.deleteGitHubPushWebhook(ctx,token,owner,name,hookID)
}

func (s *Server) deleteProjectRepositoryHooks(ctx context.Context, ownerID, projectID int64) {
	rows,err:=s.db.conn.QueryContext(ctx,`SELECT prs.device_id,prs.site_id,COALESCE(prs.github_webhook_id,0),pr.full_name,pr.integration_id FROM project_repository_sites prs JOIN project_repositories pr ON pr.project_id=prs.project_id WHERE prs.project_id=?`,projectID)
	if err!=nil{return}
	type item struct{deviceID int64;siteID string;hookID int64;fullName string;integrationID int64}
	items:=make([]item,0)
	for rows.Next(){var v item;if rows.Scan(&v.deviceID,&v.siteID,&v.hookID,&v.fullName,&v.integrationID)==nil&&v.hookID>0{items=append(items,v)}}
	rows.Close()
	for _,v:=range items{owner,name,_,parseErr:=parseGitHubRepositoryName(v.fullName);if parseErr!=nil{continue};token,tokenErr:=s.githubAppTokenForIntegration(ctx,ownerID,v.integrationID,owner);if tokenErr!=nil{continue};_ = s.deleteGitHubPushWebhook(ctx,token,owner,name,v.hookID)}
}
