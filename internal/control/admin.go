// Package control — серверная часть VELTRIX Dock (Redmi 9).
// Этот файл: API для админа (/admin/api/*). Доступ только у тебя.
package control

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"veltrix/internal/shared"
)

// ============================================================
// GET /admin/api/users
// ============================================================

// handleAdminUsers — список всех пользователей (с пагинацией).
func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage := 50
	offset := (page - 1) * perPage

	users, total, err := s.db.ListUsers(r.Context(), perPage, offset)
	if err != nil {
		apiError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":    true,
		"users": users,
		"total": total,
		"page":  page,
		"limit": perPage,
	})
}

// ============================================================
// GET /admin/api/users/{id}
// ============================================================

// handleAdminUserByID — детали пользователя или действие над ним.
// Поддерживает:
//   GET  /admin/api/users/{id}     — данные пользователя + его устройства
//   POST /admin/api/users/{id}/block   — заблокировать
//   POST /admin/api/users/{id}/unblock — разблокировать
func (s *Server) handleAdminUserByID(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/admin/api/users/")
	parts := strings.Split(rest, "/")
	if len(parts) == 0 || parts[0] == "" {
		apiError(w, shared.ErrBadRequest("invalid path"))
		return
	}

	userID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		apiError(w, shared.ErrBadRequest("invalid user id"))
		return
	}

	// GET /admin/api/users/{id}
	if r.Method == http.MethodGet && len(parts) == 1 {
		s.handleAdminGetUser(w, r, userID)
		return
	}

	// POST /admin/api/users/{id}/block
	if r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "block" {
		s.handleAdminBlock(w, r, userID)
		return
	}

	// POST /admin/api/users/{id}/unblock
	if r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "unblock" {
		s.handleAdminUnblock(w, r, userID)
		return
	}
	if r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "plan" {
		s.handleAdminSetPlan(w, r, userID)
		return
	}

	http.NotFound(w, r)
}

func (s *Server) handleAdminGetUser(w http.ResponseWriter, r *http.Request, userID int64) {
	user, err := s.db.GetUserByID(r.Context(), userID)
	if err != nil {
		apiError(w, err)
		return
	}

	user.IsAdmin = user.Username == s.adminUsername
	devices, _ := s.db.ListDevicesByUser(r.Context(), userID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"user":    user,
		"devices": devices,
	})
}

func (s *Server) handleAdminBlock(w http.ResponseWriter, r *http.Request, userID int64) {
	admin := UserFromContext(r.Context())
	if admin.ID == userID {
		apiError(w, shared.ErrBadRequest("cannot block yourself"))
		return
	}

	if err := s.db.BlockUser(r.Context(), userID); err != nil {
		apiError(w, err)
		return
	}

	// Закрываем все сессии заблокированного пользователя.
	_ = s.db.DeleteSessionsByUser(r.Context(), userID)

	// Закрываем все WebSocket-соединения его устройств.
	// (Агенты сами переподключатся, но получат "user is blocked".)
	s.kickUserDevices(userID)

	// Лог в аудит.
	s.logAdminAction(r, "block_user", userID)

	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func (s *Server) handleAdminSetPlan(w http.ResponseWriter, r *http.Request, userID int64) {
	var in struct { Plan string `json:"plan"` }
	if err := decodeJSON(r, &in); err != nil || (in.Plan != shared.UserFree && in.Plan != shared.UserPro) {
		apiError(w, shared.ErrBadRequest("plan must be free or pro"))
		return
	}
	target, err := s.db.GetUserByID(r.Context(), userID)
	if err != nil { apiError(w, err); return }
	if target.Username == s.adminUsername {
		apiError(w, shared.ErrBadRequest("the primary administrator account always has unlimited access"))
		return
	}
	res, err := s.db.conn.ExecContext(r.Context(), `UPDATE users SET status=? WHERE id=? AND status<>'blocked'`, in.Plan, userID)
	if err != nil { apiError(w, shared.ErrInternal("could not update account plan")); return }
	changed, _ := res.RowsAffected()
	if changed == 0 {
		apiError(w, shared.ErrConflict("account is blocked or plan is unchanged"))
		return
	}
	if in.Plan == shared.UserFree {
		_, _ = s.db.conn.ExecContext(r.Context(), `UPDATE api_keys SET revoked_at=UTC_TIMESTAMP() WHERE user_id=? AND revoked_at IS NULL`, userID)
	}
	s.logAdminAction(r, "account_plan_changed", userID)
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok":true,"plan":in.Plan})
}

func (s *Server) handleAdminUnblock(w http.ResponseWriter, r *http.Request, userID int64) {
	if err := s.db.UnblockUser(r.Context(), userID); err != nil {
		apiError(w, err)
		return
	}
	s.logAdminAction(r, "unblock_user", userID)
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// kickUserDevices закрывает WebSocket-соединения всех устройств пользователя.
func (s *Server) kickUserDevices(userID int64) {
	for _, conn := range s.hub.List() {
		if conn.Device.UserID == userID {
			// Закрываем соединение — агент переподключится,
			// но получит "user is blocked" при AuthRequest.
			conn.ws.Close()
		}
	}
}

// ============================================================
// GET /admin/api/stats
// ============================================================

// handleAdminStats — общая статистика для админ-панели.
func (s *Server) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats, err := s.db.GetAdminStats(r.Context())
	if err != nil {
		apiError(w, err)
		return
	}

	// Добавляем количество онлайн-устройств из хаба.
	stats["online_devices"] = len(s.hub.List())

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":    true,
		"stats": stats,
	})
}

// ============================================================
// УТИЛИТЫ
// ============================================================

// logAdminAction пишет действие админа в лог.
func (s *Server) logAdminAction(r *http.Request, action string, targetUserID int64) {
	admin := UserFromContext(r.Context())
	if admin == nil {
		return
	}
	_ = s.db.Log(r.Context(), &shared.LogEntry{
		UserID:    &admin.ID,
		Action:    action,
		Details:   map[string]interface{}{"target_user_id": targetUserID},
		IP:        clientIP(r),
		CreatedAt: time.Now().UTC(),
	})
}
