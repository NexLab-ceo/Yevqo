// Package control — серверная часть VELTRIX Dock (Redmi 9).
// Этот файл: авторизация, сессии, пароли, токены.
package control

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"veltrix/internal/shared"
)

// Константы для сессий и паролей.
const (
	SessionCookieName = "veltrix_session"
	SessionTTL        = 30 * 24 * time.Hour // 30 дней
	SessionIDBytes    = 32                  // 32 байта = 64 hex-символа
	ReferralCodeBytes = 6                   // 6 байт = 12 hex-символов
	BcryptCost        = 10                  // 10 — оптимально для Redmi 9
)

// ============================================================
// ПАРОЛИ
// ============================================================

// HashPassword хеширует пароль через bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash: %w", err)
	}
	return string(hash), nil
}

// CheckPassword проверяет, что пароль соответствует хешу.
func CheckPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// ============================================================
// ГЕНЕРАЦИЯ ТОКЕНОВ
// ============================================================

// GenerateToken генерирует случайный токен (hex).
// Используется для session ID, API-ключей и т.д.
func GenerateToken(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// GenerateReferralCode генерирует реферальный код (12 hex-символов).
// Использует crypto/rand — не Math.random.
func GenerateReferralCode() (string, error) {
	return GenerateToken(ReferralCodeBytes)
}

// ============================================================
// СЕССИИ
// ============================================================

// NewSession создаёт новую сессию для пользователя и возвращает её.
func NewSession(ctx context.Context, db *DB, userID int64, ip, userAgent string) (*shared.Session, error) {
	token, err := GenerateToken(SessionIDBytes)
	if err != nil {
		return nil, err
	}

	s := &shared.Session{
		ID:        token,
		UserID:    userID,
		IP:        ip,
		UserAgent: userAgent,
		CreatedAt: shared.Now(),
		ExpiresAt: shared.Now().Add(SessionTTL),
	}

	if err := db.CreateSession(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}

// SetSessionCookie ставит Cookie в HTTP-ответ.
func SetSessionCookie(w http.ResponseWriter, s *shared.Session, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    s.ID,
		Path:     "/",
		Expires:  s.ExpiresAt,
		HttpOnly: true,   // JS не читает — защита от XSS
		Secure:   secure, // только по HTTPS (если Cloudflare Tunnel — true)
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie удаляет Cookie (logout).
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// ============================================================
// MIDDLEWARE: АУТЕНТИФИКАЦИЯ
// ============================================================

// AuthContext — контекстные ключи, чтобы передавать пользователя и сессию
// вниз по хендлерам (через context.Context).
type authCtxKey int

const (
	ctxKeyUser authCtxKey = iota
	ctxKeySession
)

// UserFromContext достаёт пользователя из контекста (или nil).
func UserFromContext(ctx context.Context) *shared.User {
	u, _ := ctx.Value(ctxKeyUser).(*shared.User)
	return u
}

// SessionFromContext достаёт сессию из контекста (или nil).
func SessionFromContext(ctx context.Context) *shared.Session {
	s, _ := ctx.Value(ctxKeySession).(*shared.Session)
	return s
}

// RequireAuth — middleware: проверяет Cookie с сессией.
// Если сессии нет или она истекла — 401 и редирект на /login.
// Если сессия валидна — кладёт пользователя в контекст.
func (srv *Server) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil {
			srv.redirectToLogin(w, r)
			return
		}

		sess, err := srv.db.GetSession(r.Context(), cookie.Value)
		if err != nil {
			// Сессия не найдена или истекла.
			ClearSessionCookie(w)
			srv.redirectToLogin(w, r)
			return
		}

		if sess.IsExpired() {
			_ = srv.db.DeleteSession(r.Context(), sess.ID)
			ClearSessionCookie(w)
			srv.redirectToLogin(w, r)
			return
		}

		user, err := srv.db.GetUserByID(r.Context(), sess.UserID)
		if err != nil {
			ClearSessionCookie(w)
			srv.redirectToLogin(w, r)
			return
		}

		// Заблокированный пользователь — выкидываем.
		if user.Status == shared.UserBlocked {
			_ = srv.db.DeleteSession(r.Context(), sess.ID)
			ClearSessionCookie(w)
			srv.redirectToLogin(w, r)
			return
		}

		// Кладём пользователя и сессию в контекст.
		ctx := context.WithValue(r.Context(), ctxKeyUser, user)
		ctx = context.WithValue(ctx, ctxKeySession, sess)
		next(w, r.WithContext(ctx))
	}
}

// RequireAdmin — middleware: только для админа (тебя).
// Админ определяется по username из конфига (см. Server.AdminUsername).
func (srv *Server) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return srv.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil || user.Username != srv.adminUsername {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	})
}

// redirectToLogin — редирект для API (401 JSON) или для страницы (302 на /login).
func (srv *Server) redirectToLogin(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		// API — возвращаем JSON.
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"error": "unauthorized",
		})
		return
	}
	// Страница — редирект на /login.
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// ============================================================
// РЕГИСТРАЦИЯ
// ============================================================

// RegisterInput — данные для регистрации.
type RegisterInput struct {
	Username     string
	Email        string
	Password     string
	ReferralCode string // чей код ввёл (если есть)
}

// Register создаёт пользователя и возвращает его.
func (srv *Server) Register(ctx context.Context, in RegisterInput) (*shared.User, error) {
	// Валидация.
	if len(in.Username) < 3 || len(in.Username) > 64 {
		return nil, shared.ErrBadRequest("username must be 3-64 chars")
	}
	if !strings.Contains(in.Email, "@") {
		return nil, shared.ErrBadRequest("invalid email")
	}
	if len(in.Password) < 6 {
		return nil, shared.ErrBadRequest("password must be at least 6 chars")
	}

	// Хешируем пароль.
	hash, err := HashPassword(in.Password)
	if err != nil {
		return nil, err
	}

	// Генерируем реферальный код.
	referral, err := GenerateReferralCode()
	if err != nil {
		return nil, err
	}

	// Если введён чужой код — ищем пользователя.
	var referredBy *int64
	if in.ReferralCode != "" {
		if referrer, err := srv.db.GetUserByReferralCode(ctx, in.ReferralCode); err == nil {
			referredBy = &referrer.ID
		}
	}

	u := &shared.User{
		Username:     in.Username,
		Email:        in.Email,
		PasswordHash: hash,
		ReferralCode: referral,
		ReferredBy:   referredBy,
	}

	if err := srv.db.CreateUser(ctx, u); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	// Если был пригласивший — начисляем бонус.
	if referredBy != nil {
		_ = srv.db.AddBonusDevice(ctx, *referredBy, 1)
	}

	return u, nil
}

// ============================================================
// ЛОГИН
// ============================================================

// LoginInput — данные для входа.
type LoginInput struct {
	Username  string
	Password  string
	IP        string
	UserAgent string
}

// Login проверяет логин/пароль и создаёт сессию.
func (srv *Server) Login(ctx context.Context, in LoginInput) (*shared.Session, *shared.User, error) {
	user, err := srv.db.GetUserByUsername(ctx, in.Username)
	if err != nil {
		// Не раскрываем, что юзер не найден — «invalid credentials» в обоих случаях.
		return nil, nil, errors.New("invalid credentials")
	}

	if user.Status == shared.UserBlocked {
		return nil, nil, errors.New("user is blocked")
	}

	if !CheckPassword(user.PasswordHash, in.Password) {
		return nil, nil, errors.New("invalid credentials")
	}

	sess, err := NewSession(ctx, srv.db, user.ID, in.IP, in.UserAgent)
	if err != nil {
		return nil, nil, err
	}

	_ = srv.db.UpdateUserLastSeen(ctx, user.ID)

	return sess, user, nil
}

// ============================================================
// УТИЛИТЫ
// ============================================================

// writeJSON — записать JSON в ответ.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	// Игнорируем ошибку — если не получилось записать, уже ничего не сделать.
	_ = jsonNewEncoder(w).Encode(v)
}
