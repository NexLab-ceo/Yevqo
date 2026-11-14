package control

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Login throttling is stored in MariaDB so limits survive Control Plane restarts.
// Five failed attempts in a rolling one-minute window trigger a 15-minute IP block.
func (db *DB) LoginBlockedUntil(ctx context.Context, ip string) (*time.Time, error) {
	var seconds sql.NullInt64
	err := db.conn.QueryRowContext(ctx,
		"SELECT TIMESTAMPDIFF(SECOND, UTC_TIMESTAMP(), blocked_until) FROM login_attempts WHERE ip = ? AND blocked_until IS NOT NULL", ip,
	).Scan(&seconds)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read login throttle: %w", err)
	}
	if seconds.Valid && seconds.Int64 > 0 {
		until := time.Now().Add(time.Duration(seconds.Int64) * time.Second)
		return &until, nil
	}
	return nil, nil
}

func (db *DB) RecordFailedLogin(ctx context.Context, ip string) error {
	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO login_attempts (ip, window_started, attempts, blocked_until)
		VALUES (?, UTC_TIMESTAMP(), 1, NULL)
		ON DUPLICATE KEY UPDATE
			attempts = IF(window_started < UTC_TIMESTAMP() - INTERVAL 1 MINUTE, 1, attempts + 1),
			blocked_until = IF(attempts >= 5, UTC_TIMESTAMP() + INTERVAL 15 MINUTE, blocked_until),
			window_started = IF(window_started < UTC_TIMESTAMP() - INTERVAL 1 MINUTE, UTC_TIMESTAMP(), window_started)
	`, ip)
	if err != nil {
		return fmt.Errorf("record failed login: %w", err)
	}
	return nil
}

func (db *DB) ClearLoginFailures(ctx context.Context, ip string) error {
	_, err := db.conn.ExecContext(ctx, "DELETE FROM login_attempts WHERE ip = ?", ip)
	return err
}
