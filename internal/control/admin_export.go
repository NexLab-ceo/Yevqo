package control

import (
	"database/sql"
	"encoding/csv"
	"net/http"
	"strconv"
	"time"

	"veltrix/internal/shared"
)

// handleAdminUsersCSV streams a privacy-conscious user export for administrators.
// Password hashes and session credentials are deliberately excluded.
func (s *Server) handleAdminUsersCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := s.db.conn.QueryContext(r.Context(), `
		SELECT id, username, email, status, referral_code, referred_by,
		       bonus_devices, max_devices, created_at, last_seen
		FROM users ORDER BY created_at DESC, id DESC`)
	if err != nil {
		apiError(w, shared.ErrInternal("could not export users"))
		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="yevqo-users.csv"`)
	w.Header().Set("Cache-Control", "no-store, private")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"id", "username", "email", "status", "referral_code", "referred_by", "bonus_devices", "max_devices", "created_at", "last_seen"}); err != nil {
		return
	}

	count := 0
	for rows.Next() {
		var id, bonusDevices, maxDevices int64
		var username, email, status, referralCode string
		var referredBy sql.NullInt64
		var createdAt time.Time
		var lastSeen sql.NullTime
		if err := rows.Scan(&id, &username, &email, &status, &referralCode, &referredBy,
			&bonusDevices, &maxDevices, &createdAt, &lastSeen); err != nil {
			return
		}

		referredByValue := ""
		if referredBy.Valid {
			referredByValue = strconv.FormatInt(referredBy.Int64, 10)
		}
		lastSeenValue := ""
		if lastSeen.Valid {
			lastSeenValue = lastSeen.Time.UTC().Format(time.RFC3339)
		}
		record := []string{
			strconv.FormatInt(id, 10), csvSafeCell(username), csvSafeCell(email),
			csvSafeCell(status), csvSafeCell(referralCode), referredByValue,
			strconv.FormatInt(bonusDevices, 10), strconv.FormatInt(maxDevices, 10),
			createdAt.UTC().Format(time.RFC3339), lastSeenValue,
		}
		if err := writer.Write(record); err != nil {
			return
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return
	}
	writer.Flush()
	if writer.Error() != nil {
		return
	}

	if actor := UserFromContext(r.Context()); actor != nil {
		_ = s.db.Log(r.Context(), &shared.LogEntry{
			UserID: &actor.ID,
			Action: "admin_export_users_csv",
			Details: map[string]interface{}{"exported_users": count},
			IP: clientIP(r),
			CreatedAt: time.Now().UTC(),
		})
	}
}

func csvSafeCell(value string) string {
	if value == "" {
		return value
	}
	switch value[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + value
	default:
		return value
	}
}

