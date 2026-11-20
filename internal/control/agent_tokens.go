package control

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"veltrix/internal/shared"
)

// hashAgentToken stores only a one-way digest; the raw credential is returned once.
func hashAgentToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (db *DB) UserByAgentToken(ctx context.Context, token string) (*shared.User, error) {
	if len(token) != 64 {
		return nil, shared.ErrUnauthorized("invalid Agent token")
	}
	var userID int64
	err := db.conn.QueryRowContext(ctx,
		`SELECT user_id FROM agent_tokens WHERE token_hash=? AND revoked_at IS NULL`,
		hashAgentToken(token),
	).Scan(&userID)
	if err != nil {
		return nil, shared.ErrUnauthorized("invalid Agent token")
	}
	return db.GetUserByID(ctx, userID)
}

func (db *DB) RotateAgentToken(ctx context.Context, userID int64, token string) error {
	if userID < 1 || len(token) != 64 {
		return fmt.Errorf("invalid Agent token rotation request")
	}
	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO agent_tokens(user_id,token_hash,created_at,revoked_at)
		VALUES(?,?,?,NULL)
		ON DUPLICATE KEY UPDATE token_hash=VALUES(token_hash),created_at=VALUES(created_at),revoked_at=NULL
	`, userID, hashAgentToken(token), time.Now().UTC())
	return err
}

func (s *Server) handleAgentToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user := UserFromContext(r.Context())
	if user == nil {
		apiError(w, shared.ErrUnauthorized("not logged in"))
		return
	}
	token, err := GenerateToken(32)
	if err != nil {
		apiError(w, shared.ErrInternal("could not generate Agent token"))
		return
	}
	if err := s.db.RotateAgentToken(r.Context(), user.ID, token); err != nil {
		apiError(w, shared.ErrInternal("could not save Agent token"))
		return
	}
	_ = s.db.Log(r.Context(), &shared.LogEntry{
		UserID: shared.PtrInt64(user.ID),
		Action: "agent_token_rotated",
		Details: map[string]interface{}{"credential": "agent_pairing_token"},
		IP: clientIP(r),
	})
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"ok": true,
		"token": token,
		"message": "Copy this token now. It will not be shown again; previous Agent tokens are revoked.",
	})
}

