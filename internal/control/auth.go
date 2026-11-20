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

const (
	SessionCookieName = "veltrix_session"
	SessionTTL        = 30 * 24 * time.Hour
	SessionIDBytes    = 32
	ReferralCodeBytes = 6
	BcryptCost        = 10
)

// ============================================================
// ПАРОЛИ
// ============================================================

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash: %w", err)
	}
	return string(hash), nil
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// validateAccountPassword keeps registration and password changes on one policy.
// bcrypt only considers the first 72 bytes, so longer passwords are rejected
// rather than silently hashing an ambiguous prefix.
func validateAccountPassword(password string) error {
	if len(password) < 12 || len(password) > 72 || strings.TrimSpace(password) != password {
		return shared.ErrBadRequest("password must be 12-72 bytes and have no leading or trailing spaces")
	}
	return nil
}

// ============================================================
// ГЕНЕРАЦИЯ ТОКЕНОВ
// ============================================================

func GenerateToken(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func GenerateReferralCode() (string, error) {
	return GenerateToken(ReferralCodeBytes)
}

// ============================================================
// СЕССИИ
// ============================================================

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

func SetSessionCookie(w http.ResponseWriter, s *shared.Session, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    s.ID,
		Path:     "/",
		Expires:  s.ExpiresAt,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

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
// MIDDLEWARE
// ============================================================

type authCtxKey int

const (
	ctxKeyUser authCtxKey = iota
	ctxKeySession
)

func UserFromContext(ctx context.Context) *shared.User {
	u, _ := ctx.Value(ctxKeyUser).(*shared.User)
	return u
}

func SessionFromContext(ctx context.Context) *shared.Session {
	s, _ := ctx.Value(ctxKeySession).(*shared.Session)
	return s
}

func (srv *Server) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil {
			srv.redirectToLogin(w, r)
			return
		}

		sess, err := srv.db.GetSession(r.Context(), cookie.Value)
		if err != nil {
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

		if user.Status == shared.UserBlocked {
			_ = srv.db.DeleteSession(r.Context(), sess.ID)
			ClearSessionCookie(w)
			srv.redirectToLogin(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), ctxKeyUser, user)
		ctx = context.WithValue(ctx, ctxKeySession, sess)
		next(w, r.WithContext(ctx))
	}
}

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

func (srv *Server) redirectToLogin(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"error": "unauthorized",
		})
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// ============================================================
// РЕГИСТРАЦИЯ
// ============================================================

type RegisterInput struct {
	Username     string
	Email        string
	Password     string
	ReferralCode string
}

func (srv *Server) Register(ctx context.Context, in RegisterInput) (*shared.User, error) {
	if len(in.Username) < 3 || len(in.Username) > 64 {
		return nil, shared.ErrBadRequest("username must be 3-64 chars")
	}
	if !strings.Contains(in.Email, "@") {
		return nil, shared.ErrBadRequest("invalid email")
	}
	if err := validateAccountPassword(in.Password); err != nil {
		return nil, err
	}

	hash, err := HashPassword(in.Password)
	if err != nil {
		return nil, err
	}

	referral, err := GenerateReferralCode()
	if err != nil {
		return nil, err
	}

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

	if referredBy != nil {
		_ = srv.db.AddBonusDevice(ctx, *referredBy, 1)
	}

	return u, nil
}

// ============================================================
// ЛОГИН
// ============================================================

type LoginInput struct {
	Username  string
	Password  string
	IP        string
	UserAgent string
}

func (srv *Server) Login(ctx context.Context, in LoginInput) (*shared.Session, *shared.User, error) {
	user, err := srv.db.GetUserByUsername(ctx, in.Username)
	if err != nil {
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
