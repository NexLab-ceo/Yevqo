package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"veltrix/internal/shared"
)

const devicePairingTTL = 10 * time.Minute

func (db *DB) CreateDevicePairingCode(ctx context.Context, userID int64, code string, expiresAt time.Time) error {
	if userID < 1 || len(code) != 48 {
		return shared.ErrBadRequest("invalid pairing code")
	}
	_, err := db.conn.ExecContext(ctx,
		`INSERT INTO agent_pairing_codes(code_hash,user_id,expires_at) VALUES(?,?,?)`,
		hashAgentToken(code), userID, expiresAt.UTC())
	return err
}

// ClaimDevicePairingCode consumes a code exactly once and issues a credential
// scoped to one stable device UUID. Only hashes are persisted.
func (db *DB) ClaimDevicePairingCode(ctx context.Context, code, deviceID, token string) (int64, error) {
	if len(code) != 48 || len(token) != 64 || len(deviceID) < 16 || len(deviceID) > 128 {
		return 0, shared.ErrUnauthorized("invalid or expired pairing code")
	}
	for _, r := range deviceID {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return 0, shared.ErrBadRequest("invalid device identifier")
		}
	}
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil { return 0, err }
	defer tx.Rollback()

	var userID int64
	var expiresAt time.Time
	var usedAt sql.NullTime
	err = tx.QueryRowContext(ctx,
		`SELECT user_id,expires_at,used_at FROM agent_pairing_codes WHERE code_hash=? FOR UPDATE`,
		hashAgentToken(code)).Scan(&userID,&expiresAt,&usedAt)
	if errors.Is(err, sql.ErrNoRows) || err == nil && (usedAt.Valid || !expiresAt.After(time.Now().UTC())) {
		return 0, shared.ErrUnauthorized("invalid or expired pairing code")
	}
	if err != nil { return 0, err }

	var status string
	if err := tx.QueryRowContext(ctx,`SELECT status FROM users WHERE id=? FOR UPDATE`,userID).Scan(&status); err != nil {
		return 0, shared.ErrUnauthorized("invalid pairing code owner")
	}
	if status == shared.UserBlocked { return 0, shared.ErrForbidden("account is blocked") }

	var existingOwner int64
	err = tx.QueryRowContext(ctx,`SELECT user_id FROM device_agent_tokens WHERE device_id=? FOR UPDATE`,deviceID).Scan(&existingOwner)
	if err == nil && existingOwner != userID {
		return 0, shared.ErrForbidden("device identifier is already paired to another account")
	}
	if err != nil && !errors.Is(err,sql.ErrNoRows) { return 0, err }

	_, err = tx.ExecContext(ctx,`
		INSERT INTO device_agent_tokens(device_id,user_id,token_hash,created_at)
		VALUES(?,?,?,?)
		ON DUPLICATE KEY UPDATE token_hash=VALUES(token_hash),created_at=VALUES(created_at)`,
		deviceID,userID,hashAgentToken(token),time.Now().UTC())
	if err != nil { return 0, err }
	result, err := tx.ExecContext(ctx,`UPDATE agent_pairing_codes SET used_at=? WHERE code_hash=? AND used_at IS NULL`,time.Now().UTC(),hashAgentToken(code))
	if err != nil { return 0, err }
	if n, _ := result.RowsAffected(); n != 1 { return 0, shared.ErrUnauthorized("pairing code has already been used") }
	if err := tx.Commit(); err != nil { return 0, err }
	return userID,nil
}

func (db *DB) UserByDeviceAgentToken(ctx context.Context, token, deviceID string) (*shared.User, error) {
	if len(token) != 64 || deviceID == "" {
		return nil, shared.ErrUnauthorized("invalid device credential")
	}
	var userID int64
	err := db.conn.QueryRowContext(ctx,
		`SELECT user_id FROM device_agent_tokens WHERE device_id=? AND token_hash=?`,
		deviceID,hashAgentToken(token)).Scan(&userID)
	if err != nil { return nil, shared.ErrUnauthorized("invalid device credential") }
	return db.GetUserByID(ctx,userID)
}

func (s *Server) handleCreateDevicePairing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w,"method not allowed",http.StatusMethodNotAllowed); return }
	user := UserFromContext(r.Context())
	if user == nil { apiError(w,shared.ErrUnauthorized("not logged in")); return }
	code, err := GenerateToken(24)
	if err != nil { apiError(w,shared.ErrInternal("could not generate pairing code")); return }
	expires := time.Now().UTC().Add(devicePairingTTL)
	if err := s.db.CreateDevicePairingCode(r.Context(),user.ID,code,expires); err != nil {
		apiError(w,shared.ErrInternal("could not create pairing code")); return
	}
	command := "curl -fsSL https://raw.githubusercontent.com/NexLab-ceo/Yevqo/main/packaging/install-agent.sh | bash -s -- " + code
	_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(user.ID),Action:"device_pairing_code_created",Details:map[string]interface{}{"expires_in_seconds":int(devicePairingTTL.Seconds())},IP:clientIP(r)})
	writeJSON(w,http.StatusCreated,map[string]interface{}{"ok":true,"code":code,"expires_at":expires.Format(time.RFC3339),"command":command})
}

func (s *Server) handleClaimDevicePairing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w,"method not allowed",http.StatusMethodNotAllowed); return }
	r.Body = http.MaxBytesReader(w,r.Body,4096)
	var input struct { Code string `json:"code"`; DeviceID string `json:"device_id"` }
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil && !errors.Is(err,io.EOF) {
		writeJSON(w,http.StatusBadRequest,map[string]string{"error":"invalid request"}); return
	}
	input.Code = strings.TrimSpace(input.Code)
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	token, err := GenerateToken(32)
	if err != nil { writeJSON(w,http.StatusInternalServerError,map[string]string{"error":"could not issue device credential"}); return }
	userID, err := s.db.ClaimDevicePairingCode(r.Context(),input.Code,input.DeviceID,token)
	if err != nil {
		writeJSON(w,http.StatusUnauthorized,map[string]string{"error":"invalid or expired pairing code"})
		return
	}
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"token":token,"user_id":userID,"message":"Device credential issued. The pairing code is now consumed."})
}
