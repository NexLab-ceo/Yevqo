package control

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"veltrix/internal/shared"
)

func (s *Server) handleSiteWebhookSettings(w http.ResponseWriter, r *http.Request, user *shared.User, deviceID int64, siteID string) {
	dev, err := s.db.GetDeviceByID(r.Context(), deviceID)
	if err != nil { apiError(w, shared.ErrNotFound("site not found")); return }
	projectID, scopeErr := s.siteProjectScope(r.Context(), dev.UserID, deviceID, siteID)
	if scopeErr != nil { apiError(w, scopeErr); return }
	if projectID > 0 {
		action := "project.read"
		if r.Method != http.MethodGet { action = "project.manage" }
		if _, err := s.authorizeProjectOwner(r.Context(), projectID, user.ID, action); err != nil { apiError(w, err); return }
	} else {
		if err := s.authorizeWorkspace(r.Context(), dev.UserID, user.ID, "device.read"); err != nil { apiError(w, err); return }
		if r.Method != http.MethodGet {
			if err := s.authorizeWorkspace(r.Context(), dev.UserID, user.ID, "site.configure"); err != nil { apiError(w, err); return }
		}
	}
	switch r.Method {
	case http.MethodGet:
		var enabled bool
		if err := s.db.conn.QueryRowContext(r.Context(), `SELECT github_webhook_enabled FROM sites WHERE device_id=? AND site_id=?`, deviceID, siteID).Scan(&enabled); err != nil {
			apiError(w, shared.ErrInternal("could not load webhook settings")); return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "enabled": enabled, "path": fmt.Sprintf("/webhooks/github/%d/%s", deviceID, siteID)})
	case http.MethodPost:
		var in struct { Enabled bool `json:"enabled"`; AllowProjectCode bool `json:"allow_project_code"` }
		if err := decodeJSON(r, &in); err != nil { apiError(w, shared.ErrBadRequest("invalid JSON")); return }
		if !in.Enabled {
			_, err := s.db.conn.ExecContext(r.Context(), `UPDATE sites SET github_webhook_enabled=FALSE,github_webhook_secret_enc=NULL,github_webhook_updated_at=NOW() WHERE device_id=? AND site_id=?`, deviceID, siteID)
			if err != nil { apiError(w, shared.ErrInternal("could not disable GitHub webhook")); return }
			writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "enabled": false})
			return
		}
		if !in.AllowProjectCode { apiError(w, shared.ErrForbidden("explicit consent to run repository code is required before enabling automatic deployments")); return }
		var repository sqlNullString
		if err := s.db.conn.QueryRowContext(r.Context(), `SELECT git_url FROM sites WHERE device_id=? AND site_id=?`, deviceID, siteID).Scan(&repository); err != nil || !repository.Valid || strings.TrimSpace(repository.String) == "" {
			apiError(w, shared.ErrBadRequest("connect a Git repository before enabling webhooks")); return
		}
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil { apiError(w, shared.ErrInternal("could not generate webhook secret")); return }
		secret := base64.RawURLEncoding.EncodeToString(raw)
		nonce, ciphertext, err := encryptCredential(s.credentialKey, []byte(secret))
		if err != nil { apiError(w, shared.ErrInternal("could not encrypt webhook secret")); return }
		_, err = s.db.conn.ExecContext(r.Context(), `UPDATE sites SET github_webhook_secret_enc=?,github_webhook_enabled=TRUE,github_webhook_updated_at=NOW() WHERE device_id=? AND site_id=?`, append(nonce, ciphertext...), deviceID, siteID)
		if err != nil { apiError(w, shared.ErrInternal("could not save webhook configuration")); return }
		writeJSON(w, http.StatusCreated, map[string]interface{}{"ok": true, "enabled": true, "secret": secret, "path": fmt.Sprintf("/webhooks/github/%d/%s", deviceID, siteID), "message": "Copy the secret now; it will not be shown again."})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

type sqlNullString struct { String string; Valid bool }
func (n *sqlNullString) Scan(src interface{}) error {
	if src == nil { n.String = ""; n.Valid = false; return nil }
	switch v := src.(type) { case []byte: n.String = string(v); case string: n.String = v; default: return fmt.Errorf("unsupported nullable string") }
	n.Valid = true
	return nil
}

func (s *Server) handleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/webhooks/github/"), "/"), "/")
	if len(parts) != 2 || !validLogSiteID(parts[1]) { http.NotFound(w, r); return }
	deviceID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || deviceID < 1 { http.NotFound(w, r); return }
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil { http.Error(w, "payload too large or unreadable", http.StatusRequestEntityTooLarge); return }
	var encrypted []byte
	var enabled bool
	err = s.db.conn.QueryRowContext(r.Context(), `SELECT github_webhook_secret_enc,github_webhook_enabled FROM sites WHERE device_id=? AND site_id=?`, deviceID, parts[1]).Scan(&encrypted, &enabled)
	if err != nil || !enabled || len(encrypted) < 13 { http.NotFound(w, r); return }
	secret, err := decryptCredential(s.credentialKey, encrypted[:12], encrypted[12:])
	if err != nil { http.Error(w, "webhook configuration unavailable", http.StatusInternalServerError); return }
	if !validGitHubWebhookSignature(r.Header.Get("X-Hub-Signature-256"), body, secret) { http.Error(w, "invalid signature", http.StatusUnauthorized); return }

	var event struct { Ref string `json:"ref"`; After string `json:"after"`; Deleted bool `json:"deleted"` }
	if err := json.Unmarshal(body, &event); err != nil { http.Error(w, "invalid webhook payload", http.StatusBadRequest); return }
	if r.Header.Get("X-GitHub-Event") != "push" { w.WriteHeader(http.StatusAccepted); return }
	var branch string
	if err := s.db.conn.QueryRowContext(r.Context(), `SELECT git_branch FROM sites WHERE device_id=? AND site_id=?`, deviceID, parts[1]).Scan(&branch); err != nil { http.NotFound(w, r); return }
	if event.Deleted || event.Ref != "refs/heads/"+branch { w.WriteHeader(http.StatusAccepted); return }
	if !validWebhookCommit(event.After) { http.Error(w, "invalid commit", http.StatusBadRequest); return }

	var userID int64
	var sitePath, runtime string
	var port int
	if err := s.db.conn.QueryRowContext(r.Context(), `SELECT d.user_id,st.path,st.port,st.runtime FROM sites st JOIN devices d ON d.id=st.device_id WHERE st.device_id=? AND st.site_id=?`, deviceID, parts[1]).Scan(&userID, &sitePath, &port, &runtime); err != nil {
		http.NotFound(w, r); return
	}
	conn, online := s.hub.Get(deviceID)
	if !online { http.Error(w, "Agent is offline", http.StatusServiceUnavailable); return }
	gitCfg, err := s.siteGitSettings(r.Context(), deviceID, parts[1])
	if err != nil { http.Error(w, "Git settings unavailable", http.StatusInternalServerError); return }
	projectID, environmentID, scopeErr := s.siteEnvironmentScope(r.Context(), userID, deviceID, parts[1])
	if scopeErr != nil { http.Error(w, "site belongs to multiple environments; webhook deployment needs an unambiguous environment", http.StatusConflict); return }
	env, err := s.siteRuntimeEnvironment(r.Context(), userID, deviceID, parts[1], projectID, environmentID)
	if err != nil { http.Error(w, "site environment unavailable", http.StatusInternalServerError); return }
	delivery := strings.TrimSpace(r.Header.Get("X-GitHub-Delivery"))
	if delivery == "" || len(delivery) > 128 { http.Error(w, "missing delivery id", http.StatusBadRequest); return }
	res, err := s.db.conn.ExecContext(r.Context(), `INSERT IGNORE INTO github_webhook_deliveries(device_id,site_id,delivery_id,status) VALUES(?,?,?,'accepted')`, deviceID, parts[1], delivery)
	if err != nil { http.Error(w, "could not record webhook delivery", http.StatusInternalServerError); return }
	if n, _ := res.RowsAffected(); n == 0 { w.WriteHeader(http.StatusAccepted); return }
	runID := s.createWebhookDeploymentRun(r.Context(), userID, deviceID, parts[1], branch, event.After, delivery)
	_, _ = s.db.conn.ExecContext(r.Context(), `UPDATE project_repositories pr JOIN project_repository_sites prs ON prs.project_id=pr.project_id SET pr.last_commit_sha=? WHERE prs.device_id=? AND prs.site_id=?`, event.After, deviceID, parts[1])
	action := &shared.SiteAction{SiteId: parts[1], Path: sitePath, Port: uint32(port), Runtime: runtime, GitURL: gitCfg.GitURL, GitBranch: branch, GitCommit: event.After, BuildCommand: gitCfg.BuildCommand, RootDir: gitCfg.RootDir, GitToken: gitCfg.GitToken, HealthCheckPath: gitCfg.HealthCheckPath, StartCommand: gitCfg.StartCommand, AllowProjectCode: true, Env: env}
	_ = s.db.Log(r.Context(), &shared.LogEntry{UserID: shared.PtrInt64(userID), DeviceID: shared.PtrInt64(deviceID), Action: "github_webhook_accepted", Details: map[string]interface{}{"site_id": parts[1], "commit": event.After, "delivery": delivery}, IP: clientIP(r)})
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"ok": true, "accepted": true, "commit": event.After})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Hour); defer cancel()
		result, deployErr := conn.SendCommand(ctx, "deploy", action)
		status, title, message := "completed", "Deployment completed: "+parts[1], "GitHub push deployment completed."
		if deployErr != nil || result == nil || !result.Ok {
			status, title = "failed", "Deployment failed: "+parts[1]
			message = "Automatic deployment failed."
			if deployErr != nil { message = deployErr.Error() } else if result != nil && result.Message != "" { message = result.Message }
		} else {
			var started shared.StartSiteResult
			_ = shared.Unmarshal(result.Payload, &started)
			if started.CommitHash != "" { _, _ = s.db.conn.ExecContext(context.Background(), `UPDATE sites SET deployed_commit=?,deployed_at=NOW() WHERE device_id=? AND site_id=?`, started.CommitHash, deviceID, parts[1]) }
		}
		_, _ = s.db.conn.ExecContext(context.Background(), `UPDATE github_webhook_deliveries SET status=? WHERE device_id=? AND site_id=? AND delivery_id=?`, status, deviceID, parts[1], delivery)
		runStatus := "succeeded"; if status == "failed" { runStatus = "failed" }; s.finishWebhookDeploymentRun(runID, runStatus, message)
		var finalRunStatus string;_ = s.db.conn.QueryRowContext(context.Background(),`SELECT status FROM project_deployment_runs WHERE id=?`,runID).Scan(&finalRunStatus)
		if finalRunStatus=="cancelled"{title="Deployment cancelled: "+parts[1];message="Automatic deployment was cancelled.";status="cancelled"}
		if s.notificationPreferenceEnabled(context.Background(),userID,map[bool]string{true:"deployment_errors",false:"system"}[status=="failed"]) { notificationType:="success";if status=="failed"{notificationType="error"}else if status=="cancelled"{notificationType="warning"};_, _ = s.db.conn.ExecContext(context.Background(), `INSERT INTO notifications(user_id,type,title,body) VALUES(?,?,?,?)`, userID, notificationType, title, truncateNotification(message)) }
		_ = s.db.Log(context.Background(), &shared.LogEntry{UserID: shared.PtrInt64(userID), DeviceID: shared.PtrInt64(deviceID), Action: "github_webhook_deployment_"+status, Details: map[string]interface{}{"site_id": parts[1], "commit": event.After, "delivery": delivery}})
	}()
}


func validGitHubWebhookSignature(signature string, body, secret []byte) bool {
	signature = strings.TrimSpace(signature)
	if !strings.HasPrefix(signature, "sha256=") { return false }
	provided, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil || len(provided) != sha256.Size { return false }
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(body)
	return hmac.Equal(provided, mac.Sum(nil))
}

func validWebhookCommit(value string) bool {
	if len(value) != 40 && len(value) != 64 { return false }
	for _, r := range value { if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) { return false } }
	return true
}

func truncateNotification(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 1000 { return value[:1000] }
	return value
}
