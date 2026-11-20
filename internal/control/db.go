// Package control — серверная часть VELTRIX Dock (Redmi 9).
// Этот файл: подключение к MySQL и CRUD-операции.
package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"veltrix/internal/shared"
)

// DB — обёртка над *sql.DB с методами для нашего домена.
type DB struct {
	conn *sql.DB
}

// NewDB подключается к MySQL и проверяет соединение.
func NewDB(dsn string) (*DB, error) {
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping mysql: %w", err)
	}

	return &DB{conn: conn}, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) Ping(ctx context.Context) error {
	return db.conn.PingContext(ctx)
}

// ============================================================
// USERS
// ============================================================

func (db *DB) CreateUser(ctx context.Context, u *shared.User) error {
	query := `
		INSERT INTO users
			(username, email, password_hash, status, referral_code, referred_by, max_devices)
		VALUES (?, ?, ?, ?, ?, ?, ?)`

	res, err := db.conn.ExecContext(ctx, query,
		u.Username, u.Email, u.PasswordHash, shared.UserFree,
		u.ReferralCode, u.ReferredBy, 1,
	)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("last insert id: %w", err)
	}
	u.ID = id
	u.Status = shared.UserFree
	u.MaxDevices = 1
	u.CreatedAt = shared.Now()
	return nil
}

func (db *DB) GetUserByID(ctx context.Context, id int64) (*shared.User, error) {
	query := `
		SELECT id, username, email, password_hash, status,
		       referral_code, referred_by, bonus_devices, max_devices,
		       created_at, last_seen
		FROM users WHERE id = ?`

	var u shared.User
	err := db.conn.QueryRowContext(ctx, query, id).Scan(
		&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Status,
		&u.ReferralCode, &u.ReferredBy, &u.BonusDevices, &u.MaxDevices,
		&u.CreatedAt, &u.LastSeen,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, shared.ErrNotFound("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return &u, nil
}

func (db *DB) GetUserByUsername(ctx context.Context, username string) (*shared.User, error) {
	query := `
		SELECT id, username, email, password_hash, status,
		       referral_code, referred_by, bonus_devices, max_devices,
		       created_at, last_seen
		FROM users WHERE username = ?`

	var u shared.User
	err := db.conn.QueryRowContext(ctx, query, username).Scan(
		&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Status,
		&u.ReferralCode, &u.ReferredBy, &u.BonusDevices, &u.MaxDevices,
		&u.CreatedAt, &u.LastSeen,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, shared.ErrNotFound("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return &u, nil
}

func (db *DB) GetUserByReferralCode(ctx context.Context, code string) (*shared.User, error) {
	query := `
		SELECT id, username, email, password_hash, status,
		       referral_code, referred_by, bonus_devices, max_devices,
		       created_at, last_seen
		FROM users WHERE referral_code = ?`

	var u shared.User
	err := db.conn.QueryRowContext(ctx, query, code).Scan(
		&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Status,
		&u.ReferralCode, &u.ReferredBy, &u.BonusDevices, &u.MaxDevices,
		&u.CreatedAt, &u.LastSeen,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, shared.ErrNotFound("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return &u, nil
}

func (db *DB) UpdateUserLastSeen(ctx context.Context, userID int64) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE users SET last_seen = ? WHERE id = ?`,
		shared.Now(), userID,
	)
	return err
}

func (db *DB) BlockUser(ctx context.Context, userID int64) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE users SET status = ? WHERE id = ?`,
		shared.UserBlocked, userID,
	)
	return err
}

func (db *DB) UnblockUser(ctx context.Context, userID int64) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE users SET status = ? WHERE id = ?`,
		shared.UserFree, userID,
	)
	return err
}

func (db *DB) AddBonusDevice(ctx context.Context, userID int64, n int) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE users SET bonus_devices = bonus_devices + ? WHERE id = ?`,
		n, userID,
	)
	return err
}

// ============================================================
// DEVICES
// ============================================================

type deviceRowScanner interface { Scan(dest ...any) error }

func scanDevice(scanner deviceRowScanner) (*shared.Device, error) {
	var d shared.Device
	var capabilitiesJSON []byte
	err := scanner.Scan(&d.ID, &d.UserID, &d.DeviceID, &d.Hostname, &d.TailscaleIP,
		&d.AgentVersion, &d.Platform, &d.Architecture, &capabilitiesJSON,
		&d.Status, &d.CreatedAt, &d.LastSeen)
	if err != nil { return nil, err }
	if len(capabilitiesJSON) > 0 {
		if err := json.Unmarshal(capabilitiesJSON, &d.Capabilities); err != nil {
			return nil, fmt.Errorf("decode device capabilities: %w", err)
		}
	}
	return &d, nil
}

func (db *DB) UpsertDevice(ctx context.Context, d *shared.Device) (*shared.Device, error) {
	// Serialize registrations per account. Locking the user row prevents two
	// simultaneous first-time Agent connections from both passing the limit.
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil { return nil, fmt.Errorf("begin device registration: %w", err) }
	defer tx.Rollback()

	var maxDevices, bonusDevices int
	var accountStatus string
	if err := tx.QueryRowContext(ctx,
		`SELECT status, max_devices, bonus_devices FROM users WHERE id = ? FOR UPDATE`, d.UserID,
	).Scan(&accountStatus, &maxDevices, &bonusDevices); err != nil {
		if errors.Is(err, sql.ErrNoRows) { return nil, shared.ErrNotFound("user not found") }
		return nil, fmt.Errorf("lock device owner: %w", err)
	}

	existing, lookupErr := scanDevice(tx.QueryRowContext(ctx, `
		SELECT id, user_id, device_id, hostname, tailscale_ip, agent_version,
		       platform, architecture, capabilities, status, created_at, last_seen
		FROM devices WHERE device_id = ? FOR UPDATE`, d.DeviceID))
	if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
		return nil, fmt.Errorf("lookup registered device: %w", lookupErr)
	}

	now := shared.Now()
	capabilitiesJSON, err := json.Marshal(d.Capabilities)
	if err != nil { return nil, fmt.Errorf("encode device capabilities: %w", err) }
	if len(d.Capabilities) == 0 { capabilitiesJSON = nil }
	if lookupErr == nil {
		// Device UUIDs are globally unique and cannot be rebound across accounts.
		if existing.UserID != d.UserID {
			return nil, shared.ErrForbidden("device is already paired to another account")
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE devices SET tailscale_ip = ?, hostname = ?, agent_version = ?,
				platform = ?, architecture = ?, capabilities = ?, status = ?, last_seen = ?
			WHERE id = ?`,
			d.TailscaleIP, d.Hostname, d.AgentVersion, d.Platform, d.Architecture,
			capabilitiesJSON, shared.DeviceOnline, now, existing.ID)
		if err != nil { return nil, fmt.Errorf("update device: %w", err) }
		existing.TailscaleIP, existing.Hostname = d.TailscaleIP, d.Hostname
		existing.AgentVersion, existing.Platform, existing.Architecture = d.AgentVersion, d.Platform, d.Architecture
		existing.Capabilities, existing.Status, existing.LastSeen = d.Capabilities, shared.DeviceOnline, &now
		if err := tx.Commit(); err != nil { return nil, fmt.Errorf("commit device heartbeat: %w", err) }
		return existing, nil
	}

	count := 0
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM devices WHERE user_id = ?`, d.UserID).Scan(&count); err != nil {
		return nil, fmt.Errorf("count registered devices: %w", err)
	}
	limit := shared.DeviceLimitFor(accountStatus, maxDevices, bonusDevices)
	if count >= limit {
		return nil, shared.ErrForbidden("device limit reached: your current plan allows " + strconv.Itoa(limit) + " connected phone(s); disconnect a phone or upgrade your plan")
	}

	insertResult, err := tx.ExecContext(ctx, `
		INSERT INTO devices (user_id, device_id, hostname, tailscale_ip, agent_version, platform, architecture, capabilities, status, last_seen)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.UserID, d.DeviceID, d.Hostname, d.TailscaleIP, d.AgentVersion, d.Platform, d.Architecture, capabilitiesJSON, shared.DeviceOnline, now)
	if err != nil { return nil, fmt.Errorf("insert device: %w", err) }
	d.ID, err = insertResult.LastInsertId()
	if err != nil { return nil, fmt.Errorf("read registered device id: %w", err) }
	d.Status, d.CreatedAt, d.LastSeen = shared.DeviceOnline, now, &now
	if err := tx.Commit(); err != nil { return nil, fmt.Errorf("commit device registration: %w", err) }
	return d, nil
}

func (db *DB) GetDeviceByUUID(ctx context.Context, uuid string) (*shared.Device, error) {
	query := `
		SELECT id, user_id, device_id, hostname, tailscale_ip, agent_version,
		       platform, architecture, capabilities, status, created_at, last_seen
		FROM devices WHERE device_id = ?`

	d, err := scanDevice(db.conn.QueryRowContext(ctx, query, uuid))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, shared.ErrNotFound("device not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scan device: %w", err)
	}
	return d, nil
}

func (db *DB) GetDeviceByID(ctx context.Context, id int64) (*shared.Device, error) {
	query := `
		SELECT id, user_id, device_id, hostname, tailscale_ip, agent_version,
		       platform, architecture, capabilities, status, created_at, last_seen
		FROM devices WHERE id = ?`

	d, err := scanDevice(db.conn.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, shared.ErrNotFound("device not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scan device: %w", err)
	}
	return d, nil
}

// WorkspaceOwnerIDs returns the workspaces the actor can access, including their own.
func (db *DB) WorkspaceOwnerIDs(ctx context.Context, actorID int64) ([]int64, error) {
	owners := []int64{actorID}
	rows, err := db.conn.QueryContext(ctx, `SELECT owner_user_id FROM workspace_members WHERE member_user_id=? AND status='active' ORDER BY owner_user_id`, actorID)
	if err != nil { return nil, fmt.Errorf("list workspace memberships: %w", err) }
	defer rows.Close()
	for rows.Next() {
		var ownerID int64
		if err := rows.Scan(&ownerID); err != nil { return nil, err }
		owners = append(owners, ownerID)
	}
	if err := rows.Err(); err != nil { return nil, err }
	return owners, nil
}

// WorkspaceRole returns the effective role for an actor in an owner's workspace.
func (db *DB) WorkspaceRole(ctx context.Context, ownerID, actorID int64) (string, bool, error) {
	if ownerID == actorID { return "Owner", true, nil }
	var role string
	err := db.conn.QueryRowContext(ctx,
		`SELECT role FROM workspace_members WHERE owner_user_id=? AND member_user_id=? AND status='active'`,
		ownerID, actorID,
	).Scan(&role)
	if err == sql.ErrNoRows { return "", false, nil }
	if err != nil { return "", false, fmt.Errorf("lookup workspace membership: %w", err) }
	return role, true, nil
}

func (db *DB) ListDevicesByUser(ctx context.Context, userID int64) ([]shared.Device, error) {
	query := `
		SELECT id, user_id, device_id, hostname, tailscale_ip, agent_version,
		       platform, architecture, capabilities, status, created_at, last_seen
		FROM devices WHERE user_id = ?
		ORDER BY created_at DESC`

	rows, err := db.conn.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query devices: %w", err)
	}
	defer rows.Close()

	var out []shared.Device
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil { return nil, fmt.Errorf("scan device: %w", err) }
		out = append(out, *d)
	}
	return out, rows.Err()
}

func (db *DB) UpdateDeviceHeartbeat(ctx context.Context, deviceID int64) error {
	_, err := db.conn.ExecContext(ctx, "UPDATE devices SET status = ?, last_seen = ? WHERE id = ?", shared.DeviceOnline, shared.Now(), deviceID)
	return err
}

func (db *DB) MarkDeviceOffline(ctx context.Context, deviceID int64) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE devices SET status = ? WHERE id = ?`,
		shared.DeviceOffline, deviceID,
	)
	return err
}

func (db *DB) CountDevicesByUser(ctx context.Context, userID int64) (int, error) {
	var n int
	err := db.conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM devices WHERE user_id = ?`, userID,
	).Scan(&n)
	return n, err
}

func (db *DB) DeleteDevice(ctx context.Context, id int64) error {
	_, err := db.conn.ExecContext(ctx,
		`DELETE FROM devices WHERE id = ?`, id,
	)
	return err
}

// ============================================================
// SITES
// ============================================================

func (db *DB) UpsertSite(ctx context.Context, s *shared.Site) error {
	query := `
		INSERT INTO sites
			(device_id, site_id, path, port, runtime, domain, status, pid)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			path = VALUES(path),
			port = VALUES(port),
			runtime = VALUES(runtime),
			domain = VALUES(domain),
			status = VALUES(status),
			pid = VALUES(pid)`

	_, err := db.conn.ExecContext(ctx, query,
		s.DeviceID, s.SiteID, s.Path, s.Port, s.Runtime,
		s.Domain, s.Status, s.PID,
	)
	if err != nil {
		return fmt.Errorf("upsert site: %w", err)
	}
	return nil
}

func (db *DB) ListSitesByDevice(ctx context.Context, deviceID int64) ([]shared.Site, error) {
	query := `
		SELECT id, device_id, site_id, path, port, runtime, domain,
		       status, pid, created_at, updated_at
		FROM sites WHERE device_id = ?
		ORDER BY created_at ASC`

	rows, err := db.conn.QueryContext(ctx, query, deviceID)
	if err != nil {
		return nil, fmt.Errorf("query sites: %w", err)
	}
	defer rows.Close()

	var out []shared.Site
	for rows.Next() {
		var s shared.Site
		if err := rows.Scan(
			&s.ID, &s.DeviceID, &s.SiteID, &s.Path, &s.Port,
			&s.Runtime, &s.Domain, &s.Status, &s.PID,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan site: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (db *DB) UpdateSiteStatus(ctx context.Context, deviceID int64, siteID, status string, pid int) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE sites SET status = ?, pid = ? WHERE device_id = ? AND site_id = ?`,
		status, pid, deviceID, siteID,
	)
	return err
}

// ============================================================
// SESSIONS
// ============================================================

func (db *DB) CreateSession(ctx context.Context, s *shared.Session) error {
	query := `
		INSERT INTO sessions (id, user_id, ip, user_agent, expires_at)
		VALUES (?, ?, ?, ?, ?)`
	_, err := db.conn.ExecContext(ctx, query,
		s.ID, s.UserID, s.IP, s.UserAgent, s.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

func (db *DB) GetSession(ctx context.Context, id string) (*shared.Session, error) {
	query := `
		SELECT id, user_id, ip, user_agent, created_at, expires_at
		FROM sessions WHERE id = ?`

	var s shared.Session
	err := db.conn.QueryRowContext(ctx, query, id).Scan(
		&s.ID, &s.UserID, &s.IP, &s.UserAgent, &s.CreatedAt, &s.ExpiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, shared.ErrNotFound("session not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scan session: %w", err)
	}
	return &s, nil
}

func (db *DB) DeleteSession(ctx context.Context, id string) error {
	_, err := db.conn.ExecContext(ctx,
		`DELETE FROM sessions WHERE id = ?`, id,
	)
	return err
}

func (db *DB) DeleteSessionsByUser(ctx context.Context, userID int64) error {
	_, err := db.conn.ExecContext(ctx,
		`DELETE FROM sessions WHERE user_id = ?`, userID,
	)
	return err
}

func (db *DB) CleanupExpiredSessions(ctx context.Context) error {
	_, err := db.conn.ExecContext(ctx,
		`DELETE FROM sessions WHERE expires_at < ?`, shared.Now(),
	)
	return err
}

// ============================================================
// LOGS
// ============================================================

func (db *DB) Log(ctx context.Context, entry *shared.LogEntry) error {
	detailsJSON, _ := json.Marshal(entry.Details)

	query := `
		INSERT INTO logs (user_id, device_id, action, details, ip)
		VALUES (?, ?, ?, ?, ?)`
	_, err := db.conn.ExecContext(ctx, query,
		entry.UserID, entry.DeviceID, entry.Action, detailsJSON, entry.IP,
	)
	if err != nil {
		return fmt.Errorf("insert log: %w", err)
	}
	// Mirror security-relevant activity into the append-only P2 audit table.
	if entry.UserID != nil {
		ownerID := *entry.UserID
		targetType, targetID := "account", strconv.FormatInt(ownerID, 10)
		if entry.WorkspaceOwnerID != nil {
			ownerID = *entry.WorkspaceOwnerID
			targetType, targetID = "workspace", strconv.FormatInt(ownerID, 10)
		}
		if entry.DeviceID != nil {
			targetType, targetID = "device", strconv.FormatInt(*entry.DeviceID, 10)
			var deviceOwnerID int64
			if lookupErr := db.conn.QueryRowContext(ctx, `SELECT user_id FROM devices WHERE id=?`, *entry.DeviceID).Scan(&deviceOwnerID); lookupErr != nil {
				return fmt.Errorf("resolve audit workspace owner: %w", lookupErr)
			}
			if entry.WorkspaceOwnerID != nil && deviceOwnerID != ownerID {
				return fmt.Errorf("audit workspace owner does not own device")
			}
			ownerID = deviceOwnerID
		}
		outcome := "success"
		if ok, exists := entry.Details["ok"].(bool); exists && !ok { outcome = "failure" }
		action := strings.ToLower(entry.Action)
		if strings.Contains(action, "failed") || strings.Contains(action, "error") || strings.Contains(action, "denied") { outcome = "failure" }
		_, auditErr := db.conn.ExecContext(ctx, `
			INSERT INTO audit_logs (workspace_owner_id, actor_user_id, target_type, target_id, action, outcome, ip, details)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			ownerID, *entry.UserID, targetType, targetID, entry.Action, outcome, entry.IP, detailsJSON,
		)
		if auditErr != nil { return fmt.Errorf("insert audit log: %w", auditErr) }
	}
	return nil
}

// ============================================================
// ADMIN
// ============================================================

func (db *DB) ListUsers(ctx context.Context, limit, offset int) ([]shared.User, int, error) {
	var total int
	err := db.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	query := `
		SELECT id, username, email, password_hash, status,
		       referral_code, referred_by, bonus_devices, max_devices,
		       created_at, last_seen
		FROM users ORDER BY created_at DESC LIMIT ? OFFSET ?`

	rows, err := db.conn.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	var out []shared.User
	for rows.Next() {
		var u shared.User
		if err := rows.Scan(
			&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Status,
			&u.ReferralCode, &u.ReferredBy, &u.BonusDevices, &u.MaxDevices,
			&u.CreatedAt, &u.LastSeen,
		); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		out = append(out, u)
	}
	return out, total, rows.Err()
}

func (db *DB) GetAdminStats(ctx context.Context) (map[string]interface{}, error) {
	stats := map[string]interface{}{}

	var users int
	_ = db.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&users)
	stats["total_users"] = users

	var commercial int
	_ = db.conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE status = 'commercial'`).Scan(&commercial)
	stats["commercial_users"] = commercial

	var blocked int
	_ = db.conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE status = 'blocked'`).Scan(&blocked)
	stats["blocked_users"] = blocked

	var devices int
	_ = db.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM devices`).Scan(&devices)
	stats["total_devices"] = devices

	var sites int
	_ = db.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM sites`).Scan(&sites)
	stats["total_sites"] = sites

	var running int
	_ = db.conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sites WHERE status = 'running'`).Scan(&running)
	stats["running_sites"] = running

	return stats, nil
}

// ============================================================
// УТИЛИТЫ
// ============================================================

func isNotFound(err error) bool {
	var apiErr *shared.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == 404
	}
	return false
}
