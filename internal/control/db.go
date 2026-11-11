// Package control — серверная часть VELTRIX Dock (Redmi 9).
// Этот файл: подключение к MySQL и CRUD-операции.
package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"veltrix/internal/shared"
)

// DB — обёртка над *sql.DB с методами для нашего домена.
type DB struct {
	conn *sql.DB
}

// NewDB подключается к MySQL и проверяет соединение.
// DSN (Data Source Name) — строка вида:
//   user:password@tcp(127.0.0.1:3306)/veltrix?parseTime=true&charset=utf8mb4
// parseTime=true обязателен, чтобы time.Time работал.
func NewDB(dsn string) (*DB, error) {
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	// Настройки пула соединений (важно для Redmi 9 — мало RAM).
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	// Проверяем, что соединение реально работает.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping mysql: %w", err)
	}

	return &DB{conn: conn}, nil
}

// Close закрывает соединение.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Ping — проверка соединения (используется в health-check).
func (db *DB) Ping(ctx context.Context) error {
	return db.conn.PingContext(ctx)
}

// ============================================================
// USERS
// ============================================================

// CreateUser создаёт нового пользователя.
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

// GetUserByID возвращает пользователя по ID.
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

// GetUserByUsername возвращает пользователя по логину (для логина).
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

// UpdateUserLastSeen обновляет время последнего входа.
func (db *DB) UpdateUserLastSeen(ctx context.Context, userID int64) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE users SET last_seen = ? WHERE id = ?`,
		shared.Now(), userID,
	)
	return err
}

// BlockUser блокирует пользователя (админ).
func (db *DB) BlockUser(ctx context.Context, userID int64) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE users SET status = ? WHERE id = ?`,
		shared.UserBlocked, userID,
	)
	return err
}

// UnblockUser разблокирует пользователя.
func (db *DB) UnblockUser(ctx context.Context, userID int64) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE users SET status = ? WHERE id = ?`,
		shared.UserFree, userID,
	)
	return err
}

// ============================================================
// DEVICES
// ============================================================

// UpsertDevice создаёт устройство или обновляет существующее (по device_id).
// Возвращает актуальную запись (с внутренним id).
func (db *DB) UpsertDevice(ctx context.Context, d *shared.Device) (*shared.Device, error) {
	// Проверяем, есть ли устройство с таким device_id.
	existing, err := db.GetDeviceByUUID(ctx, d.DeviceID)
	if err != nil && !isNotFound(err) {
		return nil, err
	}

	if existing != nil {
		// Обновляем IP, hostname, статус, last_seen.
		query := `
			UPDATE devices
			SET tailscale_ip = ?, hostname = ?, agent_version = ?,
			    status = ?, last_seen = ?
			WHERE id = ?`
		now := shared.Now()
		_, err := db.conn.ExecContext(ctx, query,
			d.TailscaleIP, d.Hostname, d.AgentVersion,
			shared.DeviceOnline, now, existing.ID,
		)
		if err != nil {
			return nil, fmt.Errorf("update device: %w", err)
		}
		existing.TailscaleIP = d.TailscaleIP
		existing.Hostname = d.Hostname
		existing.AgentVersion = d.AgentVersion
		existing.Status = shared.DeviceOnline
		existing.LastSeen = &now
		return existing, nil
	}

	// Создаём новое.
	query := `
		INSERT INTO devices
			(user_id, device_id, hostname, tailscale_ip, agent_version, status, last_seen)
		VALUES (?, ?, ?, ?, ?, ?, ?)`
	now := shared.Now()
	res, err := db.conn.ExecContext(ctx, query,
		d.UserID, d.DeviceID, d.Hostname, d.TailscaleIP,
		d.AgentVersion, shared.DeviceOnline, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert device: %w", err)
	}
	id, _ := res.LastInsertId()
	d.ID = id
	d.Status = shared.DeviceOnline
	d.CreatedAt = now
	d.LastSeen = &now
	return d, nil
}

// GetDeviceByUUID — поиск по device_id (UUID).
func (db *DB) GetDeviceByUUID(ctx context.Context, uuid string) (*shared.Device, error) {
	query := `
		SELECT id, user_id, device_id, hostname, tailscale_ip,
		       agent_version, status, created_at, last_seen
		FROM devices WHERE device_id = ?`

	var d shared.Device
	err := db.conn.QueryRowContext(ctx, query, uuid).Scan(
		&d.ID, &d.UserID, &d.DeviceID, &d.Hostname, &d.TailscaleIP,
		&d.AgentVersion, &d.Status, &d.CreatedAt, &d.LastSeen,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, shared.ErrNotFound("device not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scan device: %w", err)
	}
	return &d, nil
}

// ListDevicesByUser — все устройства пользователя.
func (db *DB) ListDevicesByUser(ctx context.Context, userID int64) ([]shared.Device, error) {
	query := `
		SELECT id, user_id, device_id, hostname, tailscale_ip,
		       agent_version, status, created_at, last_seen
		FROM devices WHERE user_id = ?
		ORDER BY created_at DESC`

	rows, err := db.conn.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query devices: %w", err)
	}
	defer rows.Close()

	var out []shared.Device
	for rows.Next() {
		var d shared.Device
		if err := rows.Scan(
			&d.ID, &d.UserID, &d.DeviceID, &d.Hostname, &d.TailscaleIP,
			&d.AgentVersion, &d.Status, &d.CreatedAt, &d.LastSeen,
		); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// MarkDeviceOffline — вызывается, когда WebSocket-соединение закрывается.
func (db *DB) MarkDeviceOffline(ctx context.Context, deviceID int64) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE devices SET status = ? WHERE id = ?`,
		shared.DeviceOffline, deviceID,
	)
	return err
}

// CountDevicesByUser — сколько устройств у пользователя (для проверки лимита).
func (db *DB) CountDevicesByUser(ctx context.Context, userID int64) (int, error) {
	var n int
	err := db.conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM devices WHERE user_id = ?`, userID,
	).Scan(&n)
	return n, err
}

// ============================================================
// SITES
// ============================================================

// UpsertSite создаёт/обновляет сайт (по device_id + site_id).
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

// ListSitesByDevice — все сайты на устройстве.
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

// UpdateSiteStatus — обновить статус и PID (после start/stop).
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

// CreateSession создаёт сессию (Cookie для входа).
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

// GetSession — найти сессию по токену.
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

// DeleteSession — выход (logout).
func (db *DB) DeleteSession(ctx context.Context, id string) error {
	_, err := db.conn.ExecContext(ctx,
		`DELETE FROM sessions WHERE id = ?`, id,
	)
	return err
}

// CleanupExpiredSessions — удаляет просроченные сессии (cron или при старте).
func (db *DB) CleanupExpiredSessions(ctx context.Context) error {
	_, err := db.conn.ExecContext(ctx,
		`DELETE FROM sessions WHERE expires_at < ?`, shared.Now(),
	)
	return err
}

// ============================================================
// LOGS
// ============================================================

// Log записывает действие в аудит.
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
	return nil
}

// ============================================================
// УТИЛИТЫ
// ============================================================

// isNotFound — проверка, что ошибка = "не найдено".
func isNotFound(err error) bool {
	var apiErr *shared.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == 404
	}
	return false
}
// GetDeviceByID — поиск по внутреннему ID.
func (db *DB) GetDeviceByID(ctx context.Context, id int64) (*shared.Device, error) {
	query := `
		SELECT id, user_id, device_id, hostname, tailscale_ip,
		       agent_version, status, created_at, last_seen
		FROM devices WHERE id = ?`

	var d shared.Device
	err := db.conn.QueryRowContext(ctx, query, id).Scan(
		&d.ID, &d.UserID, &d.DeviceID, &d.Hostname, &d.TailscaleIP,
		&d.AgentVersion, &d.Status, &d.CreatedAt, &d.LastSeen,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, shared.ErrNotFound("device not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scan device: %w", err)
	}
	return &d, nil
}

// DeleteDevice — отвязать устройство (каскадно удалит сайты).
func (db *DB) DeleteDevice(ctx context.Context, id int64) error {
	_, err := db.conn.ExecContext(ctx,
		`DELETE FROM devices WHERE id = ?`, id,
	)
	return err
}
// ListUsers — список всех пользователей с пагинацией (для админки).
func (db *DB) ListUsers(ctx context.Context, limit, offset int) ([]shared.User, int, error) {
	// Считаем общее количество.
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

// DeleteSessionsByUser — удалить все сессии пользователя (при блокировке).
func (db *DB) DeleteSessionsByUser(ctx context.Context, userID int64) error {
	_, err := db.conn.ExecContext(ctx,
		`DELETE FROM sessions WHERE user_id = ?`, userID,
	)
	return err
}

// GetAdminStats — агрегированная статистика для админки.
func (db *DB) GetAdminStats(ctx context.Context) (map[string]interface{}, error) {
	stats := map[string]interface{}{}

	// Всего пользователей.
	var users int
	_ = db.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&users)
	stats["total_users"] = users

	// Коммерческих.
	var commercial int
	_ = db.conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE status = 'commercial'`).Scan(&commercial)
	stats["commercial_users"] = commercial

	// Заблокированных.
	var blocked int
	_ = db.conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE status = 'blocked'`).Scan(&blocked)
	stats["blocked_users"] = blocked

	// Всего устройств.
	var devices int
	_ = db.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM devices`).Scan(&devices)
	stats["total_devices"] = devices

	// Всего сайтов.
	var sites int
	_ = db.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM sites`).Scan(&sites)
	stats["total_sites"] = sites

	// Сайтов в статусе running.
	var running int
	_ = db.conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sites WHERE status = 'running'`).Scan(&running)
	stats["running_sites"] = running

	return stats, nil
}
