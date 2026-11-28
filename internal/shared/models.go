// Package shared содержит общие структуры и константы,
// используемые и сервером (cmd/control), и агентом (cmd/agent).
// Всё, что летит по сети, описано в proto/veltrix.proto.
// Всё, что хранится в памяти/БД — здесь.
package shared

import "time"

// ============================================================
// КОНСТАНТЫ (статусы, роли, действия)
// ============================================================

// UserStatus — статус аккаунта пользователя.
const (
	UserFree       = "free"
	UserCommercial = "commercial"
	UserPro        = "pro"
	UserAdvanced   = "advanced"
	UserBlocked    = "blocked"
)

// DeviceStatus — статус устройства (телефона).
const (
	DeviceOnline  = "online"
	DeviceOffline = "offline"
)

// SiteStatus — статус сайта.
const (
	SiteRunning = "running"
	SiteStopped = "stopped"
	SiteError   = "error"
)

// SiteRuntime — тип рантайма сайта.
const (
	RuntimePHP    = "php"
	RuntimeGo     = "go"
	RuntimeStatic = "static"
)

// Action — тип действия для логов (аудит).
const (
	ActionLogin       = "login"
	ActionLogout      = "logout"
	ActionRegister    = "register"
	ActionStartSite   = "start_site"
	ActionStopSite    = "stop_site"
	ActionGitPull     = "git_pull"
	ActionCreateTunnel = "create_tunnel"
	ActionBlockUser   = "block_user"
	ActionUnblockUser = "unblock_user"
)

// ============================================================
// ПОЛЬЗОВАТЕЛЬ
// ============================================================

// User — аккаунт на панели.
type User struct {
	ID           int64      `json:"id"`
	Username     string     `json:"username"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`              // никогда не отдаём в JSON
	Status       string     `json:"status"`
	IsAdmin      bool       `json:"is_admin,omitempty"`

	ReferralCode string     `json:"referral_code"`
	ReferredBy   *int64     `json:"referred_by,omitempty"`
	BonusDevices int        `json:"bonus_devices"`
	MaxDevices   int        `json:"max_devices"`

	CreatedAt    time.Time  `json:"created_at"`
	LastSeen     *time.Time `json:"last_seen,omitempty"`
}

// DeviceLimitFor returns the effective phone allowance for an account.
// Free is intentionally capped at one phone; referral bonuses never bypass it.
func DeviceLimitFor(status string, maxDevices, bonusDevices int) int {
	if status == UserFree { return 1 }
	if status == UserPro || status == UserAdvanced { return int(^uint(0) >> 1) }
	limit := maxDevices + bonusDevices
	if limit < 1 { return 1 }
	return limit
}

// MaxDevicesTotal — сколько телефонов можно подключить по тарифу.
func (u *User) MaxDevicesTotal() int {
	if u.IsAdmin { return int(^uint(0) >> 1) }
	return DeviceLimitFor(u.Status, u.MaxDevices, u.BonusDevices)
}

// ============================================================
// УСТРОЙСТВО
// ============================================================

// Device — телефон, подключённый к панели.
type Device struct {
	ID           int64      `json:"id"`
	UserID       int64      `json:"user_id"`
	DeviceID     string     `json:"device_id"`       // UUID от агента (постоянный)
	Hostname     string     `json:"hostname"`
	TailscaleIP  string     `json:"tailscale_ip"`
	AgentVersion string     `json:"agent_version"`
	Platform     string     `json:"platform,omitempty"`
	Architecture string     `json:"architecture,omitempty"`
	Capabilities []string   `json:"capabilities,omitempty"`
	Status       string     `json:"status"`

	CreatedAt    time.Time  `json:"created_at"`
	LastSeen     *time.Time `json:"last_seen,omitempty"`
}

// IsOnline — удобная проверка.
func (d *Device) IsOnline() bool {
	return d.Status == DeviceOnline
}

// ============================================================
// САЙТ
// ============================================================

// Site — сайт, запущенный на устройстве.
type Site struct {
	ID         int64      `json:"id"`
	DeviceID   int64      `json:"device_id"`   // внутренний ID из БД
	SiteID     string     `json:"site_id"`     // "blog", "shop"
	Path       string     `json:"path"`        // "sites/blog"
	Port       int        `json:"port"`
	Runtime    string     `json:"runtime"`
	Domain     string     `json:"domain,omitempty"`
	Status     string     `json:"status"`
	PID        int        `json:"pid,omitempty"`

	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// ============================================================
// СЕССИЯ (для Cookie)
// ============================================================

// Session — сессия пользователя (хранится в БД, id = токен).
type Session struct {
	ID        string    `json:"id"`          // случайный токен (hex)
	UserID    int64     `json:"user_id"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// IsExpired — проверка срока действия.
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// ============================================================
// ЛОГ (аудит)
// ============================================================

// LogEntry — одна запись в логе.
type LogEntry struct {
	ID        int64                  `json:"id"`
	UserID    *int64                 `json:"user_id,omitempty"`
	DeviceID  *int64                 `json:"device_id,omitempty"`
	WorkspaceOwnerID *int64 `json:"workspace_owner_id,omitempty"`
	Action    string                 `json:"action"`
	Details   map[string]interface{} `json:"details,omitempty"`
	IP        string                 `json:"ip,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

// ============================================================
// ОШИБКИ (для ответов API)
// ============================================================

// APIError — стандартная ошибка для JSON-ответов.
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return e.Message
}

// Новые ошибки (удобные конструкторы).
func ErrBadRequest(msg string) *APIError {
	return &APIError{Code: 400, Message: msg}
}

func ErrUnauthorized(msg string) *APIError {
	return &APIError{Code: 401, Message: msg}
}

func ErrForbidden(msg string) *APIError {
	return &APIError{Code: 403, Message: msg}
}

func ErrNotFound(msg string) *APIError {
	return &APIError{Code: 404, Message: msg}
}

func ErrConflict(msg string) *APIError {
	return &APIError{Code: 409, Message: msg}
}

func ErrInternal(msg string) *APIError {
	return &APIError{Code: 500, Message: msg}
}

// ============================================================
// УТИЛИТЫ
// ============================================================

// Now — текущее время в UTC (всегда используем UTC в БД и API).
func Now() time.Time {
	return time.Now().UTC()
}

// PtrTime — удобный хелпер: возвращает указатель на время.
func PtrTime(t time.Time) *time.Time {
	return &t
}

// PtrInt64 — удобный хелпер: возвращает указатель на int64.
func PtrInt64(v int64) *int64 {
	return &v
}
