// Package control — серверная часть VELTRIX Dock (Redmi 9).
// Этот файл: API-хендлеры для пользователей (/api/*).
package control

import (
	"context"
	"database/sql"

	"github.com/gorilla/websocket"
	"fmt"
	"encoding/json"
	"net/http"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"veltrix/internal/shared"
)

// ============================================================
// ВСПОМОГАТЕЛЬНЫЕ
// ============================================================

// writeJSON — записать JSON в ответ.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// decodeJSON читает JSON из тела запроса.
func decodeJSON(r *http.Request, v interface{}) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// apiError пишет *shared.APIError в ответ.
func apiError(w http.ResponseWriter, err error) {
	var apiErr *shared.APIError
	if e, ok := err.(*shared.APIError); ok {
		apiErr = e
	} else {
		apiErr = shared.ErrInternal("internal error")
	}
	writeJSON(w, apiErr.Code, map[string]interface{}{
		"error": apiErr.Message,
	})
}

// clientIP uses the direct TCP peer. Forwarded headers are deliberately ignored
// until an explicit trusted-proxy configuration exists.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		host = strings.Trim(r.RemoteAddr, "[]")
	}
	peer := net.ParseIP(host)
	if peer == nil {
		return "unknown"
	}
	// Cloudflared commonly connects from loopback. Only in that case accept
	// its single-hop CF-Connecting-IP header; never trust arbitrary X-Forwarded-For.
	if peer.IsLoopback() {
		if forwarded := net.ParseIP(strings.TrimSpace(r.Header.Get("CF-Connecting-IP"))); forwarded != nil {
			return forwarded.String()
		}
	}
	return peer.String()
}

// ============================================================
// POST /api/register
// ============================================================

type registerRequest struct {
	Username     string `json:"username"`
	Email        string `json:"email"`
	Password     string `json:"password"`
	ReferralCode string `json:"referral_code,omitempty"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req registerRequest
	if err := decodeJSON(r, &req); err != nil {
		apiError(w, shared.ErrBadRequest("invalid json: "+err.Error()))
		return
	}

	user, err := s.Register(r.Context(), RegisterInput{
		Username:     req.Username,
		Email:        req.Email,
		Password:     req.Password,
		ReferralCode: req.ReferralCode,
	})
	if err != nil {
		apiError(w, err)
		return
	}

	sess, _, err := s.Login(r.Context(), LoginInput{
		Username:  req.Username,
		Password:  req.Password,
		IP:        clientIP(r),
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":   true,
			"user": user,
		})
		return
	}

	SetSessionCookie(w, sess, s.cfg.UseHTTPS)
	userView := *user
	userView.IsAdmin = user.Username == s.adminUsername
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":   true,
		"user": userView,
	})
}

// ============================================================
// POST /api/login
// ============================================================

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		apiError(w, shared.ErrBadRequest("invalid json"))
		return
	}

	ip := clientIP(r)
	if ip == "unknown" {
		apiError(w, shared.ErrInternal("could not determine client IP"))
		return
	}
	if blockedUntil, err := s.db.LoginBlockedUntil(r.Context(), ip); err != nil {
		apiError(w, shared.ErrInternal("login protection unavailable"))
		return
	} else if blockedUntil != nil {
		retry := int(time.Until(*blockedUntil).Seconds())
		if retry < 1 { retry = 1 }
		w.Header().Set("Retry-After", strconv.Itoa(retry))
		writeJSON(w, http.StatusTooManyRequests, map[string]interface{}{"error":"too many failed login attempts; try again later"})
		return
	}

	sess, user, err := s.Login(r.Context(), LoginInput{
		Username:  req.Username,
		Password:  req.Password,
		IP:        ip,
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		if recordErr := s.db.RecordFailedLogin(r.Context(), ip); recordErr != nil {
			apiError(w, shared.ErrInternal("login protection unavailable"))
			return
		}
		apiError(w, shared.ErrUnauthorized("invalid credentials"))
		return
	}
	if err := s.db.ClearLoginFailures(r.Context(), ip); err != nil {
		apiError(w, shared.ErrInternal("login protection unavailable"))
		return
	}

	SetSessionCookie(w, sess, s.cfg.UseHTTPS)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":   true,
		"user": user,
	})
}

// ============================================================
// POST /api/logout
// ============================================================

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if cookie, err := r.Cookie(SessionCookieName); err == nil {
		_ = s.db.DeleteSession(r.Context(), cookie.Value)
	}

	ClearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// ============================================================
// GET /api/me
// ============================================================

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		apiError(w, shared.ErrUnauthorized("not logged in"))
		return
	}

	user.IsAdmin = user.Username == s.adminUsername
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":   true,
		"user": user,
	})
}

// GET /api/templates returns the same catalog used by the Agent scaffolder.
func (s *Server) handleAppTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "templates": shared.ListAppTemplates()})
}

// GET /api/audit-logs returns append-only workspace audit events to owners/admins.
func (s *Server) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	user := UserFromContext(r.Context())
	if user == nil { apiError(w, shared.ErrUnauthorized("not logged in")); return }
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 500 { apiError(w, shared.ErrBadRequest("limit must be between 1 and 500")); return }
		limit = parsed
	}
	projectID := int64(0)
	var owners []int64
	if raw := r.URL.Query().Get("project_id"); raw != "" {
		parsed, err := strconv.ParseInt(raw,10,64)
		if err != nil || parsed < 1 { apiError(w,shared.ErrBadRequest("invalid project id"));return }
		ownerID,accessErr:=s.authorizeProjectOwner(r.Context(),parsed,user.ID,"audit.read")
		if accessErr!=nil{apiError(w,accessErr);return}
		projectID=parsed;owners=[]int64{ownerID}
	} else {
		var err error
		owners,err=s.db.WorkspaceOwnerIDs(r.Context(),user.ID)
		if err!=nil{apiError(w,err);return}
	}
	type auditView struct {
		ID int64 `json:"id"`
		WorkspaceOwnerID int64 `json:"workspace_owner_id"`
		ActorUserID *int64 `json:"actor_user_id,omitempty"`
		TargetType string `json:"target_type"`
		TargetID string `json:"target_id"`
		Action string `json:"action"`
		Outcome string `json:"outcome"`
		RequestID string `json:"request_id"`
		IP string `json:"ip"`
		Device string `json:"device,omitempty"`
		Details json.RawMessage `json:"details,omitempty"`
		CreatedAt time.Time `json:"created_at"`
	}
	out:=make([]auditView,0,limit)
	for _,ownerID:=range owners {
		role,roleErr:=s.projectAccessRole(r.Context(),projectID,user.ID,ownerID)
		if roleErr!=nil{apiError(w,roleErr);return}
		if !CanWorkspaceAction(role,"audit.read"){continue}
		query:=`SELECT a.id,a.workspace_owner_id,a.actor_user_id,a.target_type,a.target_id,a.action,a.outcome,a.request_id,a.ip,a.details,a.created_at,COALESCE(d.hostname,'')
			FROM audit_logs a LEFT JOIN devices d ON a.target_type='device' AND d.id=CAST(a.target_id AS UNSIGNED)
			WHERE a.workspace_owner_id=?`
		args:=[]interface{}{ownerID}
		if projectID>0 {
			query+=` AND ((a.target_type='device' AND a.target_id IN (SELECT CAST(pd.device_id AS CHAR) FROM project_devices pd WHERE pd.project_id=?)) OR (a.target_type='workspace' AND CAST(JSON_UNQUOTE(JSON_EXTRACT(a.details,'$.project_id')) AS UNSIGNED)=?))`
			args=append(args,projectID,projectID)
		}
		query+=" ORDER BY a.created_at DESC,a.id DESC LIMIT ?";args=append(args,limit)
		rows,queryErr:=s.db.conn.QueryContext(r.Context(),query,args...)
		if queryErr!=nil{apiError(w,shared.ErrInternal("could not load audit log"));return}
		for rows.Next(){
			var item auditView;var actor sql.NullInt64;var details []byte
			if scanErr:=rows.Scan(&item.ID,&item.WorkspaceOwnerID,&actor,&item.TargetType,&item.TargetID,&item.Action,&item.Outcome,&item.RequestID,&item.IP,&details,&item.CreatedAt,&item.Device);scanErr!=nil{rows.Close();apiError(w,shared.ErrInternal("could not read audit log"));return}
			if actor.Valid{value:=actor.Int64;item.ActorUserID=&value}
			if len(details)>0{item.Details=append(json.RawMessage(nil),details...)}
			out=append(out,item)
		}
		if rowsErr:=rows.Err();rowsErr!=nil{rows.Close();apiError(w,shared.ErrInternal("could not read audit log"));return}
		rows.Close()
	}
	sort.Slice(out,func(i,j int)bool{return out[i].CreatedAt.After(out[j].CreatedAt)})
	if len(out)>limit{out=out[:limit]}
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"audit_logs":out})
}

// ============================================================
// GET /api/devices
// ============================================================

func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil { apiError(w, shared.ErrUnauthorized("not logged in")); return }
	projectID := int64(0)
	if raw := r.URL.Query().Get("project_id"); raw != "" {
		parsed, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || parsed < 1 { apiError(w, shared.ErrBadRequest("invalid project id")); return }
		if err := s.authorizeProjectAction(r.Context(), parsed, user.ID, "project.read"); err != nil { apiError(w, err); return }
		projectID = parsed
	}
	owners, err := s.db.WorkspaceOwnerIDs(r.Context(), user.ID)
	if err != nil { apiError(w, err); return }
	type deviceView struct {
		shared.Device
		SiteCount int `json:"site_count"`
		WorkspaceOwnerID int64 `json:"workspace_owner_id"`
		WorkspaceRole string `json:"workspace_role"`
	}
	out := make([]deviceView, 0)
	for _, ownerID := range owners {
		role, allowed, roleErr := s.db.WorkspaceRole(r.Context(), ownerID, user.ID)
		if roleErr != nil { apiError(w, roleErr); return }
		if !allowed || !CanWorkspaceAction(role, "device.read") { continue }
		devices, listErr := s.db.ListDevicesByUser(r.Context(), ownerID)
		if listErr != nil { apiError(w, listErr); return }
		for _, d := range devices {
			if projectID > 0 {
				var linked int
				if err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM project_devices WHERE project_id=? AND device_id=?`, projectID, d.ID).Scan(&linked); err != nil { apiError(w, shared.ErrInternal("could not filter project devices")); return }
				if linked == 0 { continue }
			}
			sites, _ := s.db.ListSitesByDevice(r.Context(), d.ID)
			out = append(out, deviceView{Device:d, SiteCount:len(sites), WorkspaceOwnerID:ownerID, WorkspaceRole:role})
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok":true,"devices":out,"limit":user.MaxDevicesTotal()})
}

func (s *Server) handleDeviceByID(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		apiError(w, shared.ErrUnauthorized("not logged in"))
		return
	}

	rest := strings.TrimPrefix(r.URL.Path, "/api/devices/")
	parts := strings.Split(rest, "/")
	if len(parts) == 0 || parts[0] == "" {
		apiError(w, shared.ErrBadRequest("invalid device path"))
		return
	}

	deviceID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		apiError(w, shared.ErrBadRequest("invalid device id"))
		return
	}

	dev, err := s.db.GetDeviceByID(r.Context(), deviceID)
	if err != nil {
		apiError(w, err)
		return
	}
	if err := s.authorizeWorkspace(r.Context(), dev.UserID, user.ID, "device.read"); err != nil {
		apiError(w, err)
		return
	}

	switch {
	case r.Method == http.MethodGet && len(parts) == 1:
		s.handleGetDevice(w, r, dev)
	case r.Method == http.MethodDelete && len(parts) == 1:
		if err := s.authorizeWorkspace(r.Context(), dev.UserID, user.ID, "device.delete"); err != nil {
			apiError(w, err)
			return
		}
		s.handleDeleteDevice(w, r, dev)
	case r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "cmd":
		s.handleSendCommand(w, r, dev)
	case r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "metrics":
		s.handleDeviceMetrics(w, r, dev)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleDeviceMetrics(w http.ResponseWriter, r *http.Request, dev *shared.Device) {
	conn, online := s.hub.Get(dev.ID)
	if !online { apiError(w, shared.ErrBadRequest("device is offline")); return }
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	result, err := conn.SendCommand(ctx, "get_metrics", &shared.GetMetrics{})
	if err != nil { apiError(w, shared.ErrInternal("metrics command failed")); return }
	if !result.Ok { apiError(w, shared.ErrBadRequest(result.Message)); return }
	var m shared.MetricsResult
	if err := shared.Unmarshal(result.Payload, &m); err != nil { apiError(w, shared.ErrInternal("invalid metrics response")); return }
	_, err = s.db.conn.ExecContext(r.Context(), "INSERT INTO device_metrics(device_id,cpu_percent,load_1m,memory_total_bytes,memory_available_bytes,storage_total_bytes,storage_available_bytes,temperature_celsius,captured_at) VALUES(?,?,?,?,?,?,?,?,?)",
		dev.ID,m.CPUPercent,m.Load1m,m.MemoryTotalBytes,m.MemoryAvailableBytes,m.StorageTotalBytes,m.StorageAvailableBytes,m.TemperatureCelsius,m.CapturedAt)
	if err != nil { apiError(w, shared.ErrInternal("could not store device metrics")); return }
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"metrics":m})
}

func (s *Server) handleGetDevice(w http.ResponseWriter, r *http.Request, dev *shared.Device) {
	sites, _ := s.db.ListSitesByDevice(r.Context(), dev.ID)
	_, online := s.hub.Get(dev.ID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":     true,
		"device": dev,
		"sites":  sites,
		"online": online,
	})
}

func (s *Server) handleDeleteDevice(w http.ResponseWriter, r *http.Request, dev *shared.Device) {
	// Remove remote Cloudflare routes before unpairing the device so a failed
	// cleanup cannot leave a public hostname pointing at a deleted Agent.
	rows, err := s.db.conn.QueryContext(r.Context(), `SELECT site_id FROM site_cloudflare_routes WHERE user_id=? AND device_id=?`, dev.UserID, dev.ID)
	if err != nil { apiError(w, shared.ErrInternal("could not inspect managed Cloudflare routes")); return }
	var routeSiteIDs []string
	for rows.Next() { var siteID string; if err := rows.Scan(&siteID); err != nil { rows.Close(); apiError(w, shared.ErrInternal("could not read managed Cloudflare routes")); return }; routeSiteIDs = append(routeSiteIDs,siteID) }
	if err := rows.Err(); err != nil { rows.Close(); apiError(w, shared.ErrInternal("could not read managed Cloudflare routes")); return }
	rows.Close()
	for _, siteID := range routeSiteIDs {
		if _, _, err := s.removeCloudflareRouteForSite(r.Context(),dev.UserID,dev.ID,siteID); err != nil {
			apiError(w,shared.ErrBadRequest("remove managed Cloudflare routes before unpairing this device: "+err.Error())); return
		}
	}
	// Unpairing must also terminate the live Agent socket. Otherwise a deleted
	// phone could keep receiving commands until its connection drops naturally.
	if conn, online := s.hub.Get(dev.ID); online {
		s.hub.RemoveConn(conn)
		_ = conn.ws.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "device unpaired"),
			time.Now().Add(2*time.Second))
		_ = conn.ws.Close()
	}
	if err := s.db.DeleteDevice(r.Context(), dev.ID); err != nil {
		apiError(w, err)
		return
	}
	actor:=UserFromContext(r.Context())
	_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(actor.ID),WorkspaceOwnerID:shared.PtrInt64(dev.UserID),Action:"device_unpaired",Details:map[string]interface{}{"device_id":dev.ID,"hostname":dev.Hostname},IP:clientIP(r)})
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

type sendCommandRequest struct {
	Action    string          `json:"action"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	ProjectID int64           `json:"project_id,omitempty"`
}

func (s *Server) requireProjectSite(ctx context.Context, projectID, deviceID int64, siteID string) error {
	var count int
	if err := s.db.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM project_sites WHERE project_id=? AND device_id=? AND site_id=?", projectID, deviceID, siteID).Scan(&count); err != nil { return shared.ErrInternal("could not verify project application") }
	if count != 1 { return shared.ErrForbidden("application is not assigned to this project") }
	return nil
}

func (s *Server) handleSendCommand(w http.ResponseWriter, r *http.Request, dev *shared.Device) {
	conn, online := s.hub.Get(dev.ID)
	if !online {
		apiError(w, shared.ErrBadRequest("device is offline"))
		return
	}

	var req sendCommandRequest
	if err := decodeJSON(r, &req); err != nil {
		apiError(w, shared.ErrBadRequest("invalid json"))
		return
	}

	payload, err := parsePayload(req.Action, req.Payload)
	if err != nil {
		apiError(w, shared.ErrBadRequest(err.Error()))
		return
	}
	user := UserFromContext(r.Context())
	if user == nil {
		apiError(w, shared.ErrUnauthorized("not logged in"))
		return
	}
	effectiveProjectID := req.ProjectID
	siteID := ""
	switch p := payload.(type) {
	case *shared.StartSite: siteID = p.SiteId
	case *shared.StopSite: siteID = p.SiteId
	case *shared.AndroidBuildSite: siteID = p.SiteId
	}
	if effectiveProjectID == 0 && siteID != "" {
		resolved, resolveErr := s.siteProjectScope(r.Context(), dev.UserID, dev.ID, siteID)
		if resolveErr != nil { apiError(w, resolveErr); return }
		effectiveProjectID = resolved
	}
	if effectiveProjectID > 0 {
		ownerID, authErr := s.authorizeProjectOwner(r.Context(), effectiveProjectID, user.ID, req.Action)
		if authErr != nil { apiError(w, authErr); return }
		if ownerID != dev.UserID { apiError(w, shared.ErrForbidden("device does not belong to this project workspace")); return }
		var assigned int
		if err := s.db.conn.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM project_devices WHERE project_id=? AND device_id=?", effectiveProjectID, dev.ID).Scan(&assigned); err != nil || assigned != 1 {
			apiError(w, shared.ErrForbidden("device is not assigned to this project")); return
		}
		if siteID != "" {
			if err := s.requireProjectSite(r.Context(), effectiveProjectID, dev.ID, siteID); err != nil { apiError(w, err); return }
		}
	} else if err := s.authorizeWorkspace(r.Context(), dev.UserID, user.ID, req.Action); err != nil {
		apiError(w, err)
		return
	}

	commandTimeout := 35 * time.Second
	if req.Action == "build_site" { commandTimeout = 2 * time.Hour }
	ctx, cancel := context.WithTimeout(r.Context(), commandTimeout)
	defer cancel()
	if start, ok := payload.(*shared.StartSite); ok {
		projectID, environmentID, scopeErr := s.siteEnvironmentScope(ctx, dev.UserID, dev.ID, start.SiteId)
		if scopeErr != nil { apiError(w, scopeErr); return }
		env, envErr := s.siteRuntimeEnvironment(ctx, dev.UserID, dev.ID, start.SiteId, projectID, environmentID)
		if envErr != nil { apiError(w, envErr); return }
		start.Env = env
	}
	if build, ok := payload.(*shared.AndroidBuildSite); ok {
		projectID, environmentID, scopeErr := s.siteEnvironmentScope(ctx, dev.UserID, dev.ID, build.SiteId)
		if scopeErr != nil { apiError(w, scopeErr); return }
		env, envErr := s.siteRuntimeEnvironment(ctx, dev.UserID, dev.ID, build.SiteId, projectID, environmentID)
		if envErr != nil { apiError(w, envErr); return }
		build.Environment = env
	}

	result, err := conn.SendCommand(ctx, req.Action, payload)
	if err != nil {
		apiError(w, shared.ErrInternal(err.Error()))
		return
	}

	if result.Ok {
		switch p := payload.(type) {
		case *shared.CreateSite:
			_ = s.db.UpsertSite(r.Context(), &shared.Site{DeviceID: dev.ID, SiteID: p.SiteId, Path: "sites/" + p.SiteId, Port: int(p.Port), Runtime: p.Runtime, Status: shared.SiteStopped})
		case *shared.StartSite:
			var started shared.StartSiteResult
			_ = shared.Unmarshal(result.Payload, &started)
			_ = s.db.UpsertSite(r.Context(), &shared.Site{DeviceID: dev.ID, SiteID: p.SiteId, Path: p.Path, Port: int(p.Port), Runtime: p.Runtime, Status: shared.SiteRunning, PID: int(started.Pid)})
		case *shared.StopSite:
			_ = s.db.UpdateSiteStatus(r.Context(), dev.ID, p.SiteId, shared.SiteStopped, 0)
		}
	}
	// Audit only action metadata, never raw payloads or secrets.
	details := map[string]interface{}{"ok": result.Ok}
	switch p := payload.(type) {
	case *shared.CreateSite: details["site_id"] = p.SiteId
	case *shared.StartSite: details["site_id"] = p.SiteId
	case *shared.StopSite: details["site_id"] = p.SiteId
	case *shared.GitPull: details["site_id"] = p.SiteId
	}
	_ = s.db.Log(r.Context(), &shared.LogEntry{UserID: shared.PtrInt64(user.ID), DeviceID: shared.PtrInt64(dev.ID), Action: req.Action, Details: details, IP: clientIP(r)})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":        result.Ok,
		"message":   result.Message,
		"payload":   result.Payload,
		"error_code": result.ErrorCode,
		"retryable": result.Retryable,
	})
}

func parsePayload(action string, raw json.RawMessage) (interface{}, error) {
	switch action {
	case "analyze_site_build":
		var p shared.AnalyzeSiteBuild
		if err := json.Unmarshal(raw, &p); err != nil { return nil, err }
		return &p, nil
	case "build_site":
		var p shared.AndroidBuildSite
		if err := json.Unmarshal(raw, &p); err != nil { return nil, err }
		return &p, nil
 case "list_build_history":
  var p shared.ListBuildHistory
  if err:=json.Unmarshal(raw,&p);err!=nil{return nil,err};return &p,nil
 case "get_build_history":
  var p shared.GetBuildHistory
  if err:=json.Unmarshal(raw,&p);err!=nil{return nil,err};return &p,nil
 case "cancel_build":
  return &struct{}{},nil
 case "list_build_artifacts":
  var p shared.ListBuildArtifacts
  if err:=json.Unmarshal(raw,&p);err!=nil{return nil,err};return &p,nil
 case "read_build_artifact_chunk":
  var p shared.ReadBuildArtifactChunk
  if err:=json.Unmarshal(raw,&p);err!=nil{return nil,err};return &p,nil
	case "create_site":
		var p shared.CreateSite
		if err := json.Unmarshal(raw, &p); err != nil { return nil, err }
		return &p, nil
	case "start_site":
		var p shared.StartSite
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return &p, nil
	case "stop_site":
		var p shared.StopSite
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return &p, nil
	case "git_pull":
		var p shared.GitPull
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return &p, nil
	case "update_agent":
  var p struct{}
  if err:=json.Unmarshal(raw,&p);err!=nil{return nil,err};return &p,nil
 case "restart_agent":
  var p shared.RestartAgent
  if err:=json.Unmarshal(raw,&p);err!=nil{return nil,err};return &p,nil
 case "get_device_diagnostics":
  var p shared.GetDeviceDiagnostics
  if err:=json.Unmarshal(raw,&p);err!=nil{return nil,err};return &p,nil
 case "check_device_port":
  var p shared.CheckDevicePort
  if err:=json.Unmarshal(raw,&p);err!=nil{return nil,err};return &p,nil
 case "get_logs":
		var p shared.GetLogs
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return &p, nil
	case "get_site_restart_policy":
		var p shared.GetSiteRestartPolicy
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return &p, nil
	case "set_site_restart_policy":
		var p shared.SetSiteRestartPolicy
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return &p, nil
	case "cancel_deployment_job":
		var p shared.DeploymentJobAction
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return &p, nil
	case "create_tunnel":
		var p shared.CreateTunnel
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return &p, nil
	default:
		return nil, shared.ErrBadRequest("unknown action: " + action)
	}
}

// ============================================================
// GET /api/sites
// ============================================================

func (s *Server) handleSites(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil { apiError(w, shared.ErrUnauthorized("not logged in")); return }
	projectID := int64(0)
	if raw := r.URL.Query().Get("project_id"); raw != "" {
		parsed, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || parsed < 1 { apiError(w, shared.ErrBadRequest("invalid project id")); return }
		if err := s.authorizeProjectAction(r.Context(), parsed, user.ID, "project.read"); err != nil { apiError(w, err); return }
		projectID = parsed
	}
	owners, err := s.db.WorkspaceOwnerIDs(r.Context(), user.ID)
	if err != nil { apiError(w, err); return }
	all := make([]map[string]interface{}, 0)
	for _, ownerID := range owners {
		role, allowed, roleErr := s.db.WorkspaceRole(r.Context(), ownerID, user.ID)
		if roleErr != nil { apiError(w, roleErr); return }
		if !allowed || !CanWorkspaceAction(role, "device.read") { continue }
		devices, listErr := s.db.ListDevicesByUser(r.Context(), ownerID)
		if listErr != nil { apiError(w, listErr); return }
		for _, d := range devices {
			sites, listErr := s.db.ListSitesByDevice(r.Context(), d.ID)
			if listErr != nil { apiError(w, listErr); return }
			for _, st := range sites {
				if projectID > 0 {
					var linked int
					if err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM project_sites WHERE project_id=? AND device_id=? AND site_id=?`, projectID, d.ID, st.SiteID).Scan(&linked); err != nil { apiError(w, shared.ErrInternal("could not filter project applications")); return }
					if linked == 0 { continue }
				}
				all = append(all, map[string]interface{}{"device_id":d.ID,"device_name":d.Hostname,"workspace_owner_id":ownerID,"site":st})
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok":true,"sites":all})
}

var domainPattern = regexp.MustCompile("(?i)^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\\.)+[a-z]{2,63}$")
var releaseCommitPattern = regexp.MustCompile("^(?:[a-f0-9]{40}|[a-f0-9]{64})$")

func nullableString(v string) interface{} {
	if v == "" { return nil }
	return v
}

func (s *Server) handleSiteByID(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		apiError(w, shared.ErrUnauthorized("not logged in"))
		return
	}

	rest := strings.TrimPrefix(r.URL.Path, "/api/sites/")
	parts := strings.Split(rest, "/")
	if len(parts) >= 3 && parts[2] == "previews" {
		deviceID, err := parseSiteGitRouteID(parts[0])
		if err != nil { apiError(w, err); return }
		previewKey := ""
		if len(parts) == 4 { previewKey = parts[3] } else if len(parts) != 3 { apiError(w, shared.ErrBadRequest("invalid preview route")); return }
		s.handleSitePreviews(w, r, user, deviceID, parts[1], previewKey)
		return
	}
	if len(parts) == 3 && parts[2] == "metrics" {
		deviceID, err := parseSiteGitRouteID(parts[0])
		if err != nil { apiError(w, err); return }
		dev, err := s.db.GetDeviceByID(r.Context(), deviceID)
		if err != nil { apiError(w, shared.ErrNotFound("site not found")); return }
		if err := s.authorizeWorkspace(r.Context(), dev.UserID, user.ID, "device.read"); err != nil { apiError(w, err); return }
		if !validBackupSiteID(parts[1]) { apiError(w, shared.ErrBadRequest("invalid site id")); return }
		conn, online := s.hub.Get(deviceID)
		if !online { apiError(w, shared.ErrBadRequest("device is offline")); return }
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second); defer cancel()
		result, err := conn.SendCommand(ctx, "get_site_metrics", &shared.GetSiteMetrics{SiteId:parts[1]})
		if err != nil { apiError(w, shared.ErrInternal("could not collect site metrics")); return }
		if !result.Ok { apiError(w, shared.ErrBadRequest(result.Message)); return }
		var metrics shared.SiteMetricsResult
		if err := shared.Unmarshal(result.Payload, &metrics); err != nil { apiError(w, shared.ErrInternal("invalid site metrics response")); return }
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok":true,"metrics":metrics,"scope":"main_process"})
		return
	}
	if len(parts) == 3 && parts[2] == "webhook" {
		deviceID, err := parseSiteGitRouteID(parts[0])
		if err != nil { apiError(w, err); return }
		s.handleSiteWebhookSettings(w, r, user, deviceID, parts[1])
		return
	}
	if len(parts) == 3 && parts[2] == "env" {
		deviceID, err := parseSiteGitRouteID(parts[0])
		if err != nil { apiError(w, err); return }
		s.handleSiteEnvironment(w, r, user, deviceID, parts[1])
		return
	}
	if len(parts) == 3 && parts[2] == "git" {
		deviceID, err := parseSiteGitRouteID(parts[0])
		if err != nil { apiError(w, err); return }
		s.handleSiteGit(w, r, user, deviceID, parts[1])
		return
	}
	if len(parts)==3 && parts[2]=="install-cloudflared" {
		if r.Method!=http.MethodPost { http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return }
		deviceID,err:=parseSiteGitRouteID(parts[0]);if err!=nil{apiError(w,err);return}
		dev,err:=s.db.GetDeviceByID(r.Context(),deviceID);if err!=nil{apiError(w,shared.ErrNotFound("device not found"));return};if authErr:=s.authorizeWorkspace(r.Context(),dev.UserID,user.ID,"device.configure");authErr!=nil{apiError(w,authErr);return}
		conn,online:=s.hub.Get(deviceID);if !online{apiError(w,shared.ErrBadRequest("устройство не в сети"));return}
		ctx,cancel:=context.WithTimeout(r.Context(),3*time.Minute);defer cancel()
		result,err:=conn.SendCommand(ctx,"install_cloudflared",map[string]interface{}{})
		if err!=nil||!result.Ok{msg:="Не удалось установить cloudflared";if result!=nil&&result.Message!=""{msg=result.Message};apiError(w,shared.ErrBadRequest(msg));return}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"message":"cloudflared installed"});return
	}
	if len(parts) != 2 && !(len(parts) == 3 && (parts[2] == "cmd" || parts[2] == "releases")) {
		apiError(w, shared.ErrBadRequest("expected /api/sites/{deviceID}/{siteID}[/cmd]"))
		return
	}

	deviceID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		apiError(w, shared.ErrBadRequest("invalid device id"))
		return
	}
	siteID := parts[1]

	dev, err := s.db.GetDeviceByID(r.Context(), deviceID)
	if err != nil {
		apiError(w, shared.ErrForbidden("not your device"))
		return
	}
	if err := s.authorizeWorkspace(r.Context(), dev.UserID, user.ID, "device.read"); err != nil {
		apiError(w, err)
		return
	}

	sites, _ := s.db.ListSitesByDevice(r.Context(), deviceID)
	for _, st := range sites {
		if st.SiteID != siteID { continue }
		if len(parts) == 3 && parts[2] == "releases" {
			conn, online := s.hub.Get(deviceID)
			if !online { apiError(w, shared.ErrBadRequest("device is offline")); return }
			if r.Method == http.MethodGet {
				ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second); defer cancel()
				result, err := conn.SendCommand(ctx, "list_site_releases", &shared.ListSiteReleases{SiteId:st.SiteID, Runtime:st.Runtime})
				if err != nil { apiError(w, shared.ErrInternal("could not load release history: "+err.Error())); return }
				if !result.Ok { apiError(w, shared.ErrBadRequest(result.Message)); return }
				var history shared.SiteReleasesResult
				if err := shared.Unmarshal(result.Payload, &history); err != nil { apiError(w, shared.ErrInternal("invalid release history response")); return }
				writeJSON(w, http.StatusOK, map[string]interface{}{"ok":true, "releases":history.Releases})
				return
			}
			if r.Method == http.MethodPost {
				var in struct { Keep int `json:"keep"` }
				if err := decodeJSON(r, &in); err != nil { apiError(w, shared.ErrBadRequest("invalid JSON")); return }
				if in.Keep < 1 || in.Keep > 50 { apiError(w, shared.ErrBadRequest("keep must be between 1 and 50")); return }
				if err := s.authorizeWorkspace(r.Context(), dev.UserID, user.ID, "site.configure"); err != nil { apiError(w, err); return }
				ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute); defer cancel()
				result, err := conn.SendCommand(ctx, "prune_site_releases", &shared.PruneSiteReleases{SiteId:st.SiteID, Keep:in.Keep})
				if err != nil { apiError(w, shared.ErrInternal("release cleanup failed: "+err.Error())); return }
				if !result.Ok { apiError(w, shared.ErrBadRequest(result.Message)); return }
				var pruned shared.PruneSiteReleasesResult
				if err := shared.Unmarshal(result.Payload, &pruned); err != nil { apiError(w, shared.ErrInternal("invalid release cleanup response")); return }
				_ = s.db.Log(r.Context(), &shared.LogEntry{UserID:shared.PtrInt64(user.ID), DeviceID:shared.PtrInt64(deviceID), Action:"site_release_prune", Details:map[string]interface{}{"site_id":siteID,"keep":in.Keep,"removed":len(pruned.Removed)}, IP:clientIP(r)})
				writeJSON(w, http.StatusOK, map[string]interface{}{"ok":true,"removed":pruned.Removed,"message":fmt.Sprintf("Удалено старых релизов: %d",len(pruned.Removed))})
				return
			}
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if len(parts) == 3 && parts[2] == "cmd" {
			if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
			var in struct { Action string `json:"action"`; AllowProjectCode bool `json:"allow_project_code"`; ReleaseCommit string `json:"release_commit"`; GitCommit string `json:"git_commit"`; ProjectID int64 `json:"project_id"`; EnvironmentID int64 `json:"environment_id"` }
			if err := decodeJSON(r, &in); err != nil { apiError(w, shared.ErrBadRequest("invalid JSON")); return }
			if in.ProjectID < 0 || in.EnvironmentID < 0 || (in.ProjectID == 0 && in.EnvironmentID != 0) { apiError(w, shared.ErrBadRequest("environment_id requires a project_id")); return }
			if in.Action != "deploy" && in.Action != "rebuild" && in.Action != "restart" && in.Action != "stop" && in.Action != "rollback" {
				apiError(w, shared.ErrBadRequest("unsupported site action")); return
			}
			if in.ProjectID > 0 {
				projectOwnerID, projectErr := s.authorizeProjectOwner(r.Context(), in.ProjectID, user.ID, in.Action)
				if projectErr != nil { apiError(w, projectErr); return }
				if projectOwnerID != dev.UserID { apiError(w, shared.ErrNotFound("project not found")); return }
				if projectErr := s.requireProjectSite(r.Context(), in.ProjectID, deviceID, siteID); projectErr != nil { apiError(w, projectErr); return }
				if in.EnvironmentID > 0 {
					var assigned int
					if err := s.db.conn.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM project_environment_sites WHERE project_id=? AND environment_id=? AND device_id=? AND site_id=?", in.ProjectID, in.EnvironmentID, deviceID, siteID).Scan(&assigned); err != nil || assigned != 1 {
						apiError(w, shared.ErrBadRequest("application is not assigned to the selected project environment")); return
					}
				}
			} else if err := s.authorizeWorkspace(r.Context(), dev.UserID, user.ID, in.Action); err != nil { apiError(w, err); return }
			if (in.Action == "deploy" || in.Action == "rebuild") && !in.AllowProjectCode {
				apiError(w, shared.ErrBadRequest("explicit project-code consent is required before deployment")); return
			}
			conn, online := s.hub.Get(deviceID)
			if !online { apiError(w, shared.ErrBadRequest("device is offline")); return }
			agentAction := in.Action
			payload := interface{}(&shared.SiteAction{SiteId: st.SiteID, Path: st.Path, Port: uint32(st.Port), Runtime: st.Runtime})
			if in.Action == "deploy" || in.Action == "rebuild" {
				gitCfg, cfgErr := s.siteGitSettings(r.Context(), deviceID, siteID)
				if cfgErr != nil { apiError(w, shared.ErrInternal("could not load site Git settings")); return }
				if strings.TrimSpace(gitCfg.GitURL) == "" { apiError(w, shared.ErrBadRequest("connect a Git repository before deploying")); return }
				if in.GitCommit != "" && !releaseCommitPattern.MatchString(in.GitCommit) { apiError(w, shared.ErrBadRequest("invalid Git commit SHA")); return }
				payload = &shared.SiteAction{SiteId: st.SiteID, Path: st.Path, Port: uint32(st.Port), Runtime: st.Runtime,
					GitURL: gitCfg.GitURL, GitBranch: gitCfg.GitBranch, GitCommit: in.GitCommit, BuildCommand: gitCfg.BuildCommand,
					RootDir: gitCfg.RootDir, GitToken: gitCfg.GitToken, HealthCheckPath: gitCfg.HealthCheckPath,
					CloneTimeoutSeconds: gitCfg.CloneTimeoutSeconds, BuildTimeoutSeconds: gitCfg.BuildTimeoutSeconds, HealthCheckTimeoutSeconds: gitCfg.HealthCheckTimeoutSeconds,
					StartCommand: gitCfg.StartCommand, AllowProjectCode: in.AllowProjectCode}
			}
			if in.Action == "rollback" {
				if !releaseCommitPattern.MatchString(in.ReleaseCommit) { apiError(w, shared.ErrBadRequest("invalid release commit")); return }
				agentAction = "rollback_site_release"
				gitCfg, cfgErr := s.siteGitSettings(r.Context(), deviceID, siteID)
				if cfgErr != nil { apiError(w, shared.ErrInternal("could not load site Git settings")); return }
				payload = &shared.RollbackSiteRelease{SiteId:st.SiteID, Commit:in.ReleaseCommit, RootDir:gitCfg.RootDir, Runtime:st.Runtime, Port:uint32(st.Port), StartCommand:gitCfg.StartCommand}
			}
			if in.Action == "stop" {
				agentAction = "stop_site"
				payload = &shared.StopSite{SiteId: st.SiteID}
			}
			switch scoped := payload.(type) {
			case *shared.SiteAction, *shared.RollbackSiteRelease:
				projectID, environmentID := in.ProjectID, in.EnvironmentID
				if projectID == 0 || environmentID == 0 {
					resolvedProjectID, resolvedEnvironmentID, scopeErr := s.siteEnvironmentScope(r.Context(), dev.UserID, deviceID, siteID)
					if scopeErr != nil { apiError(w, scopeErr); return }
					if projectID == 0 { projectID = resolvedProjectID }
					if environmentID == 0 { environmentID = resolvedEnvironmentID }
				}
				// A project can own an application that has not yet been assigned
				// to an environment; in that case no project-level variables apply.
				if environmentID == 0 { projectID = 0 }
				env, envErr := s.siteRuntimeEnvironment(r.Context(), dev.UserID, deviceID, siteID, projectID, environmentID)
				if envErr != nil { apiError(w, envErr); return }
				switch target := scoped.(type) {
				case *shared.SiteAction: target.Env = env
				case *shared.RollbackSiteRelease: target.Env = env
				}
			}
			commandTimeout := 2 * time.Minute
			if in.Action == "deploy" || in.Action == "rebuild" {
				// Git checkout and the local build provider can legitimately take
				// much longer than a normal lifecycle command.
				commandTimeout = 25 * time.Minute
			}
			ctx, cancel := context.WithTimeout(r.Context(), commandTimeout)
			defer cancel()
			result, err := conn.SendCommand(ctx, agentAction, payload)
			if err != nil { apiError(w, shared.ErrInternal("site command failed: " + err.Error())); return }
			if !result.Ok {
				// Failed deployments are first-class history entries. Keep the
				// active site status unless the deployment rollback also failed.
				rollbackFailed := strings.Contains(result.Message, "rollback also failed")
				deployAttempt := in.Action == "deploy" || in.Action == "rebuild"
				if in.Action != "stop" && (!deployAttempt || rollbackFailed) {
					_ = s.db.UpdateSiteStatus(r.Context(), deviceID, siteID, shared.SiteError, 0)
				}
				if deployAttempt || in.Action == "rollback" {
					message := strings.TrimSpace(result.Message)
					if len(message) > 900 { message = message[:900] }
					_ = s.db.Log(r.Context(), &shared.LogEntry{UserID: shared.PtrInt64(user.ID), DeviceID: shared.PtrInt64(deviceID), Action: "site_" + in.Action, Details: map[string]interface{}{"site_id": siteID, "ok": false, "error": message}, IP: clientIP(r)})
					s.notifyWorkspaceEvent(r.Context(), dev.UserID, user.ID, "error", "Deployment failed: "+siteID, message)
				}
				writeJSON(w, http.StatusOK, map[string]interface{}{"ok": false, "message": result.Message})
				return
			}
			if in.Action == "stop" {
				_ = s.db.UpdateSiteStatus(r.Context(), deviceID, siteID, shared.SiteStopped, 0)
			} else {
				var started shared.StartSiteResult
				_ = shared.Unmarshal(result.Payload, &started)
				_ = s.db.UpdateSiteStatus(r.Context(), deviceID, siteID, shared.SiteRunning, int(started.Pid))
				if (in.Action == "deploy" || in.Action == "rebuild" || in.Action == "rollback") && (started.CommitHash != "" || in.ReleaseCommit != "") {
					_, _ = s.db.conn.ExecContext(r.Context(), "UPDATE sites SET deployed_commit=?,deployed_at=NOW() WHERE device_id=? AND site_id=?", started.CommitHash, deviceID, siteID)
				}
			}
			logDetails := map[string]interface{}{"site_id": siteID, "ok": true, "action": in.Action}
			if in.Action == "deploy" || in.Action == "rebuild" || in.Action == "rollback" {
				var started shared.StartSiteResult
				_ = shared.Unmarshal(result.Payload, &started)
				if started.CommitHash == "" && in.Action == "rollback" { started.CommitHash = in.ReleaseCommit }
				if started.CommitHash != "" { logDetails["commit_hash"] = started.CommitHash }
				if started.CommitDate != "" { logDetails["commit_date"] = started.CommitDate }
			}
			_ = s.db.Log(r.Context(), &shared.LogEntry{UserID: shared.PtrInt64(user.ID), DeviceID: shared.PtrInt64(deviceID), Action: "site_" + in.Action, Details: logDetails, IP: clientIP(r)})
			if in.Action == "deploy" || in.Action == "rebuild" || in.Action == "rollback" {
				commit := fmt.Sprint(logDetails["commit_hash"])
				body := "Deployment completed successfully."
				if commit != "" && commit != "<nil>" { body += " Commit: " + commit }
				s.notifyWorkspaceEvent(r.Context(), dev.UserID, user.ID, "success", "Deployment completed: "+siteID, body)
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": result.Message})
			return
		}
		if r.Method == http.MethodPatch {
			if err := s.authorizeWorkspace(r.Context(), dev.UserID, user.ID, "site.configure"); err != nil { apiError(w, err); return }
			var in struct { Domain string `json:"domain"`; TunnelToken string `json:"tunnel_token"` }
			if err:=decodeJSON(r,&in);err!=nil{apiError(w,shared.ErrBadRequest("invalid JSON"));return}
			domain:=strings.ToLower(strings.TrimSpace(in.Domain))
			var configuredHostname string
			routeErr:=s.db.conn.QueryRowContext(r.Context(),`SELECT hostname FROM site_cloudflare_routes WHERE user_id=? AND device_id=? AND site_id=?`,dev.UserID,deviceID,siteID).Scan(&configuredHostname)
			if routeErr==nil && !strings.EqualFold(configuredHostname,domain) { apiError(w,shared.ErrConflict("remove the managed Cloudflare route before changing or removing the site domain")); return }
			if routeErr!=nil && routeErr!=sql.ErrNoRows { apiError(w,shared.ErrInternal("could not check Cloudflare route")); return }
			if domain!="" && !domainPattern.MatchString(domain){apiError(w,shared.ErrBadRequest("enter a valid hostname, such as app.example.com"));return}
			if domain=="" {
				conn,online:=s.hub.Get(deviceID)
				if !online { apiError(w,shared.ErrBadRequest("device is offline; reconnect the Agent before removing the active tunnel")); return }
				ctx,cancel:=context.WithTimeout(r.Context(),20*time.Second);defer cancel()
				stopped,stopErr:=conn.SendCommand(ctx,"stop_tunnel",&shared.StopTunnel{SiteId:siteID})
				if stopErr!=nil || stopped==nil || !stopped.Ok { msg:="could not stop Cloudflare Tunnel";if stopped!=nil&&stopped.Message!=""{msg=stopped.Message};apiError(w,shared.ErrBadRequest(msg));return }
				_,err:=s.db.conn.ExecContext(r.Context(),"UPDATE sites SET domain=NULL,tunnel_token_enc=NULL,tunnel_status='inactive',tunnel_error=NULL WHERE device_id=? AND site_id=?",deviceID,siteID)
				if err!=nil{apiError(w,shared.ErrInternal("could not remove domain"));return}
				writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"domain":"","tunnel_status":"inactive"});return
			}
			// Keep token encrypted at rest. If omitted, reuse a previously saved token.
			var encrypted interface{}
			if strings.TrimSpace(in.TunnelToken)!="" {
				if len(in.TunnelToken)>4096 || strings.ContainsAny(in.TunnelToken,"\\r\\n\\x00"){apiError(w,shared.ErrBadRequest("invalid Cloudflare tunnel token"));return}
				nonce,ciphertext,e:=encryptCredential(s.credentialKey,[]byte(strings.TrimSpace(in.TunnelToken)))
				if e!=nil{apiError(w,shared.ErrInternal("could not encrypt tunnel token"));return}
				encrypted=append(nonce,ciphertext...)
				_,e=s.db.conn.ExecContext(r.Context(),"UPDATE sites SET domain=?,tunnel_token_enc=?,tunnel_status='pending',tunnel_error=NULL,tunnel_updated_at=NOW() WHERE device_id=? AND site_id=?",domain,encrypted,deviceID,siteID)
				if e!=nil{apiError(w,shared.ErrInternal("could not save domain"));return}
			}else{
				_,e:=s.db.conn.ExecContext(r.Context(),"UPDATE sites SET domain=?,tunnel_status='pending',tunnel_error=NULL,tunnel_updated_at=NOW() WHERE device_id=? AND site_id=?",domain,deviceID,siteID)
				if e!=nil{apiError(w,shared.ErrInternal("could not save domain"));return}
			}
			var tokenEnc []byte
			if e:=s.db.conn.QueryRowContext(r.Context(),"SELECT tunnel_token_enc FROM sites WHERE device_id=? AND site_id=?",deviceID,siteID).Scan(&tokenEnc);e!=nil||len(tokenEnc)<13{
				_,_=s.db.conn.ExecContext(r.Context(),"UPDATE sites SET tunnel_status='error',tunnel_error=? WHERE device_id=? AND site_id=?","Укажите Cloudflare Tunnel token",deviceID,siteID)
				writeJSON(w,http.StatusOK,map[string]interface{}{"ok":false,"domain":domain,"tunnel_status":"error","message":"Укажите Cloudflare Tunnel token"});return
			}
			token,e:=decryptCredential(s.credentialKey,tokenEnc[:12],tokenEnc[12:])
			if e!=nil{apiError(w,shared.ErrInternal("could not decrypt Cloudflare token"));return}
			conn,online:=s.hub.Get(deviceID)
			if !online{_,_=s.db.conn.ExecContext(r.Context(),"UPDATE sites SET tunnel_status='error',tunnel_error=? WHERE device_id=? AND site_id=?","Устройство не в сети",deviceID,siteID);writeJSON(w,http.StatusOK,map[string]interface{}{"ok":false,"domain":domain,"tunnel_status":"error","message":"Устройство не в сети"});return}
			ctx,cancel:=context.WithTimeout(r.Context(),45*time.Second);defer cancel()
			result,e:=conn.SendCommand(ctx,"create_tunnel",&shared.CreateTunnel{SiteId:siteID,Hostname:domain,TunnelToken:string(token),Port:uint32(st.Port)})
			if e!=nil||!result.Ok{
				msg:="cloudflared не запущен"
				if result!=nil&&result.Message!=""{msg=result.Message}
				_,_=s.db.conn.ExecContext(r.Context(),"UPDATE sites SET tunnel_status='error',tunnel_error=?,tunnel_updated_at=NOW() WHERE device_id=? AND site_id=?",msg,deviceID,siteID)
				writeJSON(w,http.StatusOK,map[string]interface{}{"ok":false,"domain":domain,"tunnel_status":"error","message":msg});return
			}
			_,_=s.db.conn.ExecContext(r.Context(),"UPDATE sites SET tunnel_status='active',tunnel_error=NULL,tunnel_updated_at=NOW() WHERE device_id=? AND site_id=?",deviceID,siteID)
			writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"domain":domain,"tunnel_status":"active","message":"Cloudflare Tunnel connected"});return
		}
		if r.Method == http.MethodDelete {
			if st.Domain != "" {
				conn, online := s.hub.Get(deviceID)
				if !online { apiError(w, shared.ErrBadRequest("device is offline; reconnect the Agent before deleting a site with an active domain")); return }
				ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
				defer cancel()
				result, err := conn.SendCommand(ctx, "stop_tunnel", &shared.StopTunnel{SiteId: siteID})
				if err != nil || result == nil || !result.Ok { apiError(w, shared.ErrBadRequest("could not stop Cloudflare Tunnel; site metadata was not removed")); return }
			}
			if st.Status == shared.SiteRunning {
				conn, online := s.hub.Get(deviceID)
				if !online { apiError(w, shared.ErrBadRequest("device is offline; stop the site before removing it")); return }
				ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
				defer cancel()
				result, err := conn.SendCommand(ctx, "stop_site", &shared.StopSite{SiteId: siteID})
				if err != nil || !result.Ok { apiError(w, shared.ErrBadRequest("could not stop site; metadata was not removed")); return }
			}
			if _, _, routeErr := s.removeCloudflareRouteForSite(r.Context(), dev.UserID, deviceID, siteID); routeErr != nil { apiError(w, shared.ErrBadRequest("remove the managed Cloudflare route before deleting this site: "+routeErr.Error())); return }
			_, _ = s.db.conn.ExecContext(r.Context(), "DELETE FROM project_sites WHERE device_id=? AND site_id=?", deviceID, siteID)
			// Remove only the link; the independent database and its data remain intact.
			_, _ = s.db.conn.ExecContext(r.Context(), "DELETE FROM site_database_connections WHERE device_id=? AND site_id=?", deviceID, siteID)
			if _, err := s.db.conn.ExecContext(r.Context(), "DELETE FROM sites WHERE device_id=? AND site_id=?", deviceID, siteID); err != nil { apiError(w, shared.ErrInternal("could not remove site metadata")); return }
			writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "Site metadata removed. Device files were preserved."})
			return
		}
		if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "site": st})
		return
	}

	apiError(w, shared.ErrNotFound("site not found"))
}
