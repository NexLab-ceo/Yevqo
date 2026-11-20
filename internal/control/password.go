package control

import (
	"net/http"

	"veltrix/internal/shared"
)

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user := UserFromContext(r.Context())
	if user == nil {
		apiError(w, shared.ErrUnauthorized("not logged in"))
		return
	}
	var req changePasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		apiError(w, shared.ErrBadRequest("invalid JSON"))
		return
	}
	if !CheckPassword(user.PasswordHash, req.CurrentPassword) {
		apiError(w, shared.ErrUnauthorized("current password is incorrect"))
		return
	}
	if err := validateAccountPassword(req.NewPassword); err != nil {
		apiError(w, err)
		return
	}
	if req.NewPassword == req.CurrentPassword {
		apiError(w, shared.ErrBadRequest("new password must differ from current password"))
		return
	}
	hash, err := HashPassword(req.NewPassword)
	if err != nil {
		apiError(w, shared.ErrInternal("could not secure new password"))
		return
	}
	if _, err := s.db.conn.ExecContext(r.Context(), "UPDATE users SET password_hash = ? WHERE id = ?", hash, user.ID); err != nil {
		apiError(w, shared.ErrInternal("could not update password"))
		return
	}

	// Revoke every previous session and issue one fresh session to this browser.
	if err := s.db.DeleteSessionsByUser(r.Context(), user.ID); err != nil {
		apiError(w, shared.ErrInternal("password changed; please sign in again"))
		ClearSessionCookie(w)
		return
	}
	session, err := NewSession(r.Context(), s.db, user.ID, clientIP(r), r.UserAgent())
	if err != nil {
		ClearSessionCookie(w)
		apiError(w, shared.ErrInternal("password changed; please sign in again"))
		return
	}
	SetSessionCookie(w, session, s.cfg.UseHTTPS)
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "password changed"})
}
