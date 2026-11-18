package control

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"veltrix/internal/shared"
)

var siteEnvKeyPattern = regexp.MustCompile(`^[A-Z_][A-Z0-9_]{0,63}$`)

func validSiteEnvKey(key string) bool {
	if !siteEnvKeyPattern.MatchString(key) { return false }
	// Do not let app-level values replace process execution or managed database
	// configuration. DB credentials are injected through database connections.
	switch key {
	case "PATH", "HOME", "PREFIX", "TMPDIR", "LD_PRELOAD", "LD_LIBRARY_PATH", "PORT", "HOST",
		"DB_CONNECTION", "DB_HOST", "DB_PORT", "DB_SSLMODE", "DB_DATABASE", "DB_USERNAME", "DB_PASSWORD", "VELTRIX_DATABASES":
		return false
	}
	return !strings.HasPrefix(key, "DB_") && !strings.HasPrefix(key, "VELTRIX_")
}

func (s *Server) handleSiteEnvironment(w http.ResponseWriter, r *http.Request, user *shared.User, deviceID int64, siteID string) {
	dev, err := s.db.GetDeviceByID(r.Context(), deviceID)
	if err != nil { apiError(w, shared.ErrNotFound("site not found")); return }
	if err := s.authorizeWorkspace(r.Context(), dev.UserID, user.ID, "device.read"); err != nil {
		apiError(w, err)
		return
	}

	switch r.Method {
	case http.MethodGet:
		rows, err := s.db.conn.QueryContext(r.Context(), `SELECT name FROM site_environment_vars WHERE user_id=? AND device_id=? AND site_id=? ORDER BY name`, dev.UserID, deviceID, siteID)
		if err != nil { apiError(w, shared.ErrInternal("could not load environment variables")); return }
		defer rows.Close()
		items := make([]map[string]interface{}, 0)
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil { apiError(w, shared.ErrInternal("could not read environment variables")); return }
			items = append(items, map[string]interface{}{"name": name, "masked": true})
		}
		if err := rows.Err(); err != nil { apiError(w, shared.ErrInternal("could not read environment variables")); return }
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "variables": items})
	case http.MethodPost:
		var in struct { Name string `json:"name"`; Value string `json:"value"` }
		if err := decodeJSON(r, &in); err != nil { apiError(w, shared.ErrBadRequest("invalid JSON")); return }
		in.Name = strings.TrimSpace(in.Name)
		if !validSiteEnvKey(in.Name) { apiError(w, shared.ErrBadRequest("environment key is invalid or reserved")); return }
		if len(in.Value) > 16384 || strings.IndexByte(in.Value, 0) >= 0 { apiError(w, shared.ErrBadRequest("environment value must be at most 16 KiB and cannot contain NUL")); return }
		nonce, ciphertext, err := encryptCredential(s.credentialKey, []byte(in.Value))
		if err != nil { apiError(w, shared.ErrInternal("could not encrypt environment value")); return }
		_, err = s.db.conn.ExecContext(r.Context(), `INSERT INTO site_environment_vars(user_id,device_id,site_id,name,nonce,ciphertext) VALUES(?,?,?,?,?,?) ON DUPLICATE KEY UPDATE nonce=VALUES(nonce),ciphertext=VALUES(ciphertext),updated_at=CURRENT_TIMESTAMP`, dev.UserID, deviceID, siteID, in.Name, nonce, ciphertext)
		if err != nil { apiError(w, shared.ErrInternal("could not save environment variable")); return }
		_ = s.db.Log(r.Context(), &shared.LogEntry{UserID: shared.PtrInt64(user.ID), DeviceID: shared.PtrInt64(deviceID), Action: "site_environment_updated", Details: map[string]interface{}{"site_id": siteID, "key": in.Name}, IP: clientIP(r)})
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "name": in.Name, "masked": true})
	case http.MethodDelete:
		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if !validSiteEnvKey(name) { apiError(w, shared.ErrBadRequest("environment key is invalid")); return }
		res, err := s.db.conn.ExecContext(r.Context(), `DELETE FROM site_environment_vars WHERE user_id=? AND device_id=? AND site_id=? AND name=?`, dev.UserID, deviceID, siteID, name)
		if err != nil { apiError(w, shared.ErrInternal("could not delete environment variable")); return }
		n, _ := res.RowsAffected()
		if n == 0 { apiError(w, shared.ErrNotFound("environment variable not found")); return }
		_ = s.db.Log(r.Context(), &shared.LogEntry{UserID: shared.PtrInt64(user.ID), DeviceID: shared.PtrInt64(deviceID), Action: "site_environment_deleted", Details: map[string]interface{}{"site_id": siteID, "key": name}, IP: clientIP(r)})
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) siteEnvironment(ctx context.Context, userID, deviceID int64, siteID string) (map[string]string, error) {
	rows, err := s.db.conn.QueryContext(ctx, `SELECT name,nonce,ciphertext FROM site_environment_vars WHERE user_id=? AND device_id=? AND site_id=? ORDER BY name`, userID, deviceID, siteID)
	if err != nil { return nil, err }
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var name string
		var nonce, ciphertext []byte
		if err := rows.Scan(&name, &nonce, &ciphertext); err != nil { return nil, err }
		if !validSiteEnvKey(name) { return nil, shared.ErrInternal("stored environment key is invalid") }
		value, err := decryptCredential(s.credentialKey, nonce, ciphertext)
		if err != nil { return nil, err }
		out[name] = string(value)
	}
	return out, rows.Err()
}

func (s *Server) siteRuntimeEnvironment(ctx context.Context, userID, deviceID int64, siteID string) (map[string]string, error) {
	custom, err := s.siteEnvironment(ctx, userID, deviceID, siteID)
	if err != nil { return nil, err }
	database, err := s.siteDatabaseEnvironment(ctx, userID, deviceID, siteID)
	if err != nil { return nil, err }
	// Database connection values are authoritative and cannot be shadowed by
	// custom variables; credentials remain encrypted at rest.
	for key, value := range database { custom[key] = value }
	return custom, nil
}

func parseSiteEnvironmentDeviceID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 { return 0, shared.ErrBadRequest("invalid device id") }
	return id, nil
}
