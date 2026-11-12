// Package control — серверная часть VELTRIX Dock (Redmi 9).
// Этот файл: API-хендлеры для пользователей (/api/*).
package control

import (
	"context"
	"encoding/json"
	"net/http"
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

// clientIP вытаскивает IP из запроса (учитывает X-Forwarded-For).
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	return strings.Split(r.RemoteAddr, ":")[0]
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
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":   true,
		"user": user,
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

	sess, user, err := s.Login(r.Context(), LoginInput{
		Username:  req.Username,
		Password:  req.Password,
		IP:        clientIP(r),
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		apiError(w, shared.ErrUnauthorized(err.Error()))
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
	default:
		http.NotFound(w, r)
	}
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

	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()

	result, err := conn.SendCommand(ctx, req.Action, payload)
	if err != nil {
		apiError(w, shared.ErrInternal(err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      result.Ok,
		"message": result.Message,
		"payload": result.Payload,
	})
}

func parsePayload(action string, raw json.RawMessage) (interface{}, error) {
	switch action {
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

func (s *Server) handleSiteByID(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		apiError(w, shared.ErrUnauthorized("not logged in"))
		return
	}

	rest := strings.TrimPrefix(r.URL.Path, "/api/sites/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 {
		apiError(w, shared.ErrBadRequest("expected /api/sites/{deviceID}/{siteID}"))
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
		if st.SiteID == siteID {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"ok":   true,
				"site": st,
			})
			return
		}
	}

	apiError(w, shared.ErrNotFound("site not found"))
}
