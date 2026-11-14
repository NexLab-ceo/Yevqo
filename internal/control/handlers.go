// Package control — серверная часть VELTRIX Dock (Redmi 9).
// Этот файл: API-хендлеры для пользователей (/api/*).
package control

import (
	"context"
	"encoding/json"
	"net/http"
	"net"
	"regexp"
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

// ============================================================
// GET /api/devices
// ============================================================

func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		apiError(w, shared.ErrUnauthorized("not logged in"))
		return
	}

	devices, err := s.db.ListDevicesByUser(r.Context(), user.ID)
	if err != nil {
		apiError(w, err)
		return
	}

	type deviceView struct {
		shared.Device
		SiteCount int `json:"site_count"`
	}
	out := make([]deviceView, 0, len(devices))
	for _, d := range devices {
		sites, _ := s.db.ListSitesByDevice(r.Context(), d.ID)
		out = append(out, deviceView{Device: d, SiteCount: len(sites)})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"devices": out,
		"limit":   user.MaxDevicesTotal(),
	})
}

// ============================================================
// /api/devices/{id}
// ============================================================

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
	if dev.UserID != user.ID {
		apiError(w, shared.ErrForbidden("not your device"))
		return
	}

	switch {
	case r.Method == http.MethodGet && len(parts) == 1:
		s.handleGetDevice(w, r, dev)
	case r.Method == http.MethodDelete && len(parts) == 1:
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
	if err := s.db.DeleteDevice(r.Context(), dev.ID); err != nil {
		apiError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

type sendCommandRequest struct {
	Action  string          `json:"action"`
	Payload json.RawMessage `json:"payload,omitempty"`
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

	commandTimeout := 35 * time.Second
	if req.Action == "build_site" { commandTimeout = 2 * time.Hour }
	ctx, cancel := context.WithTimeout(r.Context(), commandTimeout)
	defer cancel()
	if start, ok := payload.(*shared.StartSite); ok {
		env, envErr := s.siteDatabaseEnvironment(ctx, dev.UserID, dev.ID, start.SiteId)
		if envErr != nil { apiError(w, shared.ErrInternal("could not prepare protected database environment")); return }
		start.Env = env
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
	_ = s.db.Log(r.Context(), &shared.LogEntry{UserID: shared.PtrInt64(dev.UserID), DeviceID: shared.PtrInt64(dev.ID), Action: req.Action, Details: details, IP: clientIP(r)})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      result.Ok,
		"message": result.Message,
		"payload": result.Payload,
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
	case "get_logs":
		var p shared.GetLogs
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
	if user == nil {
		apiError(w, shared.ErrUnauthorized("not logged in"))
		return
	}

	devices, _ := s.db.ListDevicesByUser(r.Context(), user.ID)
	all := make([]map[string]interface{}, 0)
	for _, d := range devices {
		sites, _ := s.db.ListSitesByDevice(r.Context(), d.ID)
		for _, st := range sites {
			all = append(all, map[string]interface{}{
				"device_id":   d.ID,
				"device_name": d.Hostname,
				"site":        st,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":    true,
		"sites": all,
	})
}

// ============================================================
// /api/sites/{deviceID}/{siteID}
// ============================================================

var domainPattern = regexp.MustCompile("(?i)^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\\.)+[a-z]{2,63}$")

func nullableString(v string) interface{} { if v == "" { return nil }; return v }

func (s *Server) handleSiteByID(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		apiError(w, shared.ErrUnauthorized("not logged in"))
		return
	}

	rest := strings.TrimPrefix(r.URL.Path, "/api/sites/")
	parts := strings.Split(rest, "/")
	if len(parts) == 3 && parts[2] == "git" {
		deviceID, err := parseSiteGitRouteID(parts[0])
		if err != nil { apiError(w, err); return }
		s.handleSiteGit(w, r, user, deviceID, parts[1])
		return
	}
	if len(parts)==3 && parts[2]=="install-cloudflared" {
		if r.Method!=http.MethodPost { http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return }
		deviceID,err:=parseSiteGitRouteID(parts[0]);if err!=nil{apiError(w,err);return}
		dev,err:=s.db.GetDeviceByID(r.Context(),deviceID);if err!=nil||dev.UserID!=user.ID{apiError(w,shared.ErrForbidden("not your device"));return}
		conn,online:=s.hub.Get(deviceID);if !online{apiError(w,shared.ErrBadRequest("устройство не в сети"));return}
		ctx,cancel:=context.WithTimeout(r.Context(),3*time.Minute);defer cancel()
		result,err:=conn.SendCommand(ctx,"install_cloudflared",map[string]interface{}{})
		if err!=nil||!result.Ok{msg:="Не удалось установить cloudflared";if result!=nil&&result.Message!=""{msg=result.Message};apiError(w,shared.ErrBadRequest(msg));return}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"message":"cloudflared installed"});return
	}
	if len(parts) != 2 && !(len(parts) == 3 && parts[2] == "cmd") {
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
	if err != nil || dev.UserID != user.ID {
		apiError(w, shared.ErrForbidden("not your device"))
		return
	}

	sites, _ := s.db.ListSitesByDevice(r.Context(), deviceID)
	for _, st := range sites {
		if st.SiteID != siteID { continue }
		if len(parts) == 3 && parts[2] == "cmd" {
			if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
			var in struct { Action string `json:"action"` }
			if err := decodeJSON(r, &in); err != nil { apiError(w, shared.ErrBadRequest("invalid JSON")); return }
			if in.Action != "deploy" && in.Action != "rebuild" && in.Action != "restart" && in.Action != "stop" {
				apiError(w, shared.ErrBadRequest("unsupported site action")); return
			}
			conn, online := s.hub.Get(deviceID)
			if !online { apiError(w, shared.ErrBadRequest("device is offline")); return }
			agentAction := in.Action
			payload := interface{}(&shared.SiteAction{SiteId: st.SiteID, Path: st.Path, Port: uint32(st.Port), Runtime: st.Runtime})
			if in.Action == "deploy" || in.Action == "rebuild" {
				gitCfg, cfgErr := s.siteGitSettings(r.Context(), deviceID, siteID)
				if cfgErr != nil { apiError(w, shared.ErrInternal("could not load site Git settings")); return }
				if strings.TrimSpace(gitCfg.GitURL) == "" { apiError(w, shared.ErrBadRequest("connect a Git repository before deploying")); return }
				payload = &shared.SiteAction{SiteId: st.SiteID, Path: st.Path, Port: uint32(st.Port), Runtime: st.Runtime,
					GitURL: gitCfg.GitURL, GitBranch: gitCfg.GitBranch, BuildCommand: gitCfg.BuildCommand,
					RootDir: gitCfg.RootDir, GitToken: gitCfg.GitToken}
			}
			if in.Action == "stop" {
				agentAction = "stop_site"
				payload = &shared.StopSite{SiteId: st.SiteID}
			}
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
			defer cancel()
			result, err := conn.SendCommand(ctx, agentAction, payload)
			if err != nil { apiError(w, shared.ErrInternal("site command failed: " + err.Error())); return }
			if !result.Ok {
				if in.Action != "stop" { _ = s.db.UpdateSiteStatus(r.Context(), deviceID, siteID, shared.SiteError, 0) }
				writeJSON(w, http.StatusOK, map[string]interface{}{"ok": false, "message": result.Message})
				return
			}
			if in.Action == "stop" {
				_ = s.db.UpdateSiteStatus(r.Context(), deviceID, siteID, shared.SiteStopped, 0)
			} else {
				var started shared.StartSiteResult
				_ = shared.Unmarshal(result.Payload, &started)
				_ = s.db.UpdateSiteStatus(r.Context(), deviceID, siteID, shared.SiteRunning, int(started.Pid))
				if (in.Action == "deploy" || in.Action == "rebuild") && started.CommitHash != "" {
					_, _ = s.db.conn.ExecContext(r.Context(), "UPDATE sites SET deployed_commit=?,deployed_at=NOW() WHERE device_id=? AND site_id=?", started.CommitHash, deviceID, siteID)
				}
			}
			_ = s.db.Log(r.Context(), &shared.LogEntry{UserID: shared.PtrInt64(user.ID), DeviceID: shared.PtrInt64(deviceID), Action: "site_" + in.Action, Details: map[string]interface{}{"site_id": siteID, "ok": true}, IP: clientIP(r)})
			writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": result.Message})
			return
		}
		if r.Method == http.MethodPatch {
			var in struct { Domain string `json:"domain"`; TunnelToken string `json:"tunnel_token"` }
			if err:=decodeJSON(r,&in);err!=nil{apiError(w,shared.ErrBadRequest("invalid JSON"));return}
			domain:=strings.ToLower(strings.TrimSpace(in.Domain))
			if domain!="" && !domainPattern.MatchString(domain){apiError(w,shared.ErrBadRequest("enter a valid hostname, such as app.example.com"));return}
			if domain=="" {
				_,err:=s.db.conn.ExecContext(r.Context(),"UPDATE sites SET domain=NULL,tunnel_status='inactive',tunnel_error=NULL WHERE device_id=? AND site_id=?",deviceID,siteID)
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
			if st.Status == shared.SiteRunning {
				conn, online := s.hub.Get(deviceID)
				if !online { apiError(w, shared.ErrBadRequest("device is offline; stop the site before removing it")); return }
				ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
				defer cancel()
				result, err := conn.SendCommand(ctx, "stop_site", &shared.StopSite{SiteId: siteID})
				if err != nil || !result.Ok { apiError(w, shared.ErrBadRequest("could not stop site; metadata was not removed")); return }
			}
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
