// Package control — серверная часть VELTRIX Dock (Redmi 9).
// Этот файл: HTTP-сервер, роутинг, middleware, статика.
package control

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config — настройки сервера (из config.json).
type Config struct {
	ListenAddr    string `json:"listen_addr"`     // ":8080"
	DataRoot      string `json:"data_root"`       // "/sdcard/yevqo"
	StaticDir     string `json:"static_dir"`      // "public" (SvelteKit)
	MySQLDSN      string `json:"mysql_dsn"`       // user:pass@tcp(...)/veltrix
	AdminUsername string `json:"admin_username"`  // "maksim"
	UseHTTPS      bool   `json:"use_https"`       // true, если за Cloudflare
}

// Server — главная структура сервера.
type Server struct {
	cfg           Config
	db            *DB
	hub           *Hub
	logStreams    *logStreamHub
	adminUsername string
	credentialKey []byte
	schedulerStop chan struct{}
	schedulerDone chan struct{}
	mux           *http.ServeMux
}

// NewServer создаёт сервер, подключается к БД, применяет миграции, инициализирует хаб.
func NewServer(cfg Config) (*Server, error) {
	// Проверяем, что DataRoot существует.
	if err := os.MkdirAll(cfg.DataRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create data_root: %w", err)
	}

	credentialKey, err := loadCredentialKey(cfg.DataRoot)
	if err != nil { return nil, fmt.Errorf("load database credential key: %w", err) }

	// Создаём собственную базу Control Plane, если она ещё не существует.
	// MariaDB уже запущена установщиком; локальный root доступ используется
	// только для CREATE DATABASE. Все рабочие запросы идут через отдельную учётную запись из MySQLDSN.
	bootstrapCtx, bootstrapCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := ensureMySQLDatabase(bootstrapCtx, cfg.MySQLDSN); err != nil {
		bootstrapCancel()
		return nil, fmt.Errorf("bootstrap Control Plane database: %w", err)
	}
	bootstrapCancel()

	// Подключаемся к собственной базе под обычной учётной записью приложения.
	db, err := NewDB(cfg.MySQLDSN)
	if err != nil {
		return nil, fmt.Errorf("connect mysql: %w", err)
	}

	// Применяем миграции, если папка существует.
	// Это делается ДО запуска HTTP-сервера — если миграция упадёт,
	// сервер вообще не стартует (лучше упасть на старте, чем на живой БД).
	migrationsDir := filepath.Join(cfg.DataRoot, "migrations")
	if _, err := os.Stat(migrationsDir); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := db.RunMigrations(ctx, migrationsDir)
		cancel()
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("migrations: %w", err)
		}
		log.Printf("migrations applied from %s", migrationsDir)
	} else {
		db.Close()
		return nil, fmt.Errorf("required migrations directory is missing: %s: %w", migrationsDir, err)
	}

	// Чистим просроченные сессии при старте (не критично, если упадёт).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_ = db.CleanupExpiredSessions(ctx)
	cancel()

	srv := &Server{
		cfg:           cfg,
		db:            db,
		hub:           NewHub(),
		logStreams:    newLogStreamHub(),
		adminUsername: cfg.AdminUsername,
		credentialKey: credentialKey,
		mux:           http.NewServeMux(),
	}

	srv.schedulerStop = make(chan struct{})
	srv.schedulerDone = make(chan struct{})
	srv.registerRoutes()
	go srv.backupSchedulerLoop()
	return srv, nil
}
// Close закрывает ресурсы.
func (s *Server) Close() error {
	if s.schedulerStop != nil {
		close(s.schedulerStop)
		<-s.schedulerDone
	}
	return s.db.Close()
}

// ============================================================
// РОУТИНГ
// ============================================================

// registerRoutes настраивает все маршруты.
func (s *Server) registerRoutes() {
	// --- WebSocket для агентов ---
	s.mux.HandleFunc("/ws/logs", s.RequireAuth(s.handleLogStream))
	s.mux.HandleFunc("/ws", s.handleWebSocket)
	s.mux.HandleFunc("/webhooks/github/", s.handleGitHubWebhook)

	// --- API для пользователей ---
	s.mux.HandleFunc("/api/register", s.handleRegister)
	s.mux.HandleFunc("/api/login", s.handleLogin)
	s.mux.HandleFunc("/api/password", s.RequireAuth(s.handleChangePassword))
	s.mux.HandleFunc("/api/logout", s.handleLogout)
	s.mux.HandleFunc("/api/me", s.RequireAuth(s.handleMe))
	s.mux.HandleFunc("/api/agent-token", s.RequireAuth(s.handleAgentToken))
	s.mux.HandleFunc("/api/templates", s.RequireAuth(s.handleAppTemplates))
	s.mux.HandleFunc("/api/workspaces", s.RequireAuth(s.handleWorkspaces))
	s.mux.HandleFunc("/api/workspace/members", s.RequireAuth(s.handleWorkspaceMembers))
	s.mux.HandleFunc("/api/workspace/members/", s.RequireAuth(s.handleWorkspaceMemberByID))
	s.mux.HandleFunc("/api/workspace/invitations", s.RequireAuth(s.handleWorkspaceInvitations))
	s.mux.HandleFunc("/api/devices", s.RequireAuth(s.handleDevices))
	s.mux.HandleFunc("/api/devices/", s.RequireAuth(s.handleDeviceByID))
	s.mux.HandleFunc("/api/sites", s.RequireAuth(s.handleSites))
	s.mux.HandleFunc("/api/sites/", s.RequireAuth(s.handleSiteByID))
	// Workspace and operations modules.
	s.mux.HandleFunc("/api/projects", s.RequireAuth(s.handleProjects))
	s.mux.HandleFunc("/api/projects/", s.RequireAuth(func(w http.ResponseWriter, r *http.Request) { if strings.Contains(r.URL.Path, "/repository") { s.handleProjectRepository(w, r); return }; if strings.Contains(r.URL.Path, "/deployments") { s.handleProjectDeployments(w, r); return }; if strings.Contains(r.URL.Path, "/environments") { s.handleProjectEnvironments(w, r); return }; if strings.Contains(r.URL.Path, "/members") { s.handleProjectMembers(w, r); return }; s.handleProjectByID(w, r) }))
	s.mux.HandleFunc("/api/databases/servers", s.RequireAuth(s.handleDatabaseServers))
	s.mux.HandleFunc("/api/databases/servers/", s.RequireAuth(s.handleDatabaseServerByID))
	s.mux.HandleFunc("/api/databases", s.RequireAuth(s.handleDatabaseCollection))
	s.mux.HandleFunc("/api/databases/", s.RequireAuth(func(w http.ResponseWriter,r *http.Request) { if strings.HasSuffix(r.URL.Path,"/test") { s.handleDatabaseConnectionTest(w,r); return }; if strings.HasSuffix(r.URL.Path,"/stats") { s.handleDatabaseStats(w,r); return }; s.handleDatabaseByID(w,r) }))
	s.mux.HandleFunc("/api/database-connections", s.RequireAuth(s.handleDatabaseConnections))
	s.mux.HandleFunc("/api/logs", s.RequireAuth(s.handleLogs))
	s.mux.HandleFunc("/api/audit-logs", s.RequireAuth(s.handleAuditLogs))
	s.mux.HandleFunc("/api/notifications", s.RequireAuth(s.handleNotifications))
	s.mux.HandleFunc("/api/settings", s.RequireAuth(s.handleSettings))
	s.mux.HandleFunc("/api/reports", s.RequireAuth(s.handleReports))
	s.mux.HandleFunc("/api/backups", s.RequireAuth(s.handleBackups))
	s.mux.HandleFunc("/api/backup-schedules", s.RequireAuth(s.handleBackupSchedules))
	s.mux.HandleFunc("/api/backup-schedules/", s.RequireAuth(s.handleBackupSchedules))
	s.mux.HandleFunc("/api/scheduled-tasks", s.RequireAuth(s.handleScheduledTasks))
	s.mux.HandleFunc("/api/scheduled-tasks/", s.RequireAuth(s.handleScheduledTasks))
	s.mux.HandleFunc("/api/backups/", s.RequireAuth(func(w http.ResponseWriter,r *http.Request) { if strings.HasSuffix(r.URL.Path,"/transfer") { s.handleBackupTransfer(w,r); return }; s.handleDatabaseBackupChunk(w,r) }))
	s.mux.HandleFunc("/api/backups/restore", s.RequireAuth(s.handleRestoreDatabaseBackup))
	s.mux.HandleFunc("/api/backups/restore-site", s.RequireAuth(s.handleRestoreSiteBackup))
	s.mux.HandleFunc("/api/domains", s.RequireAuth(s.handleDomains))
	s.mux.HandleFunc("/api/deployments", s.RequireAuth(s.handleDeployments))
	s.mux.HandleFunc("/api/monitoring", s.RequireAuth(s.handleMonitoring))
	s.mux.HandleFunc("/api/integrations/github-app/config", s.RequireAuth(s.handleGitHubAppConfig))
	s.mux.HandleFunc("/api/integrations/github-app/state", s.RequireAuth(s.handleGitHubAppState))
	s.mux.HandleFunc("/api/integrations", s.RequireAuth(s.handleIntegrations))
	s.mux.HandleFunc("/api/integrations/", s.RequireAuth(func(w http.ResponseWriter, r *http.Request) { if strings.HasSuffix(r.URL.Path, "/repositories") { s.handleIntegrationRepositories(w, r); return }; s.handleIntegrationByID(w, r) }))
	s.mux.HandleFunc("/api/cloudflare", s.RequireAuth(s.handleCloudflareConfig))
	s.mux.HandleFunc("/api/cloudflare/dns", s.RequireAuth(s.handleCloudflareDNS))
	s.mux.HandleFunc("/api/cloudflare/tunnel-routes", s.RequireAuth(s.handleCloudflareTunnelRoutes))

	// --- API для админа ---
	s.mux.HandleFunc("/admin/api/users/export.csv", s.RequireAdmin(s.handleAdminUsersCSV))
	s.mux.HandleFunc("/admin/api/users", s.RequireAdmin(s.handleAdminUsers))
	s.mux.HandleFunc("/admin/api/users/", s.RequireAdmin(s.handleAdminUserByID))
	s.mux.HandleFunc("/admin/api/stats", s.RequireAdmin(s.handleAdminStats))
	s.mux.HandleFunc("/admin/api/backups/control-plane", s.RequireAdmin(s.handleControlPlaneBackup))

	// --- Статика SvelteKit (последний маршрут — fallback) ---
	s.mux.HandleFunc("/", s.handleStatic)
}

// ============================================================
// HTTP-СЕРВЕР
// ============================================================

// Start запускает HTTP-сервер.
func (s *Server) Start() error {
	addr := s.cfg.ListenAddr
	if addr == "" {
		addr = ":8080"
	}

	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           s.loggingMiddleware(s.mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("VELTRIX Dock listening on %s", addr)
	return httpSrv.ListenAndServe()
}

// ============================================================
// MIDDLEWARE
// ============================================================

// loggingMiddleware логирует запросы (метод, путь, статус, время).
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r)
		dur := time.Since(start)

		// Не логируем WebSocket и статику, чтобы не засорять лог.
		if r.URL.Path != "/ws" && !strings.HasPrefix(r.URL.Path, "/_app/") {
			log.Printf("%s %s -> %d (%s)",
				r.Method, r.URL.Path, rw.status, dur.Round(time.Millisecond))
		}
	})
}

// responseWriter оборачивает http.ResponseWriter, чтобы запомнить статус.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Hijack forwards WebSocket upgrade requests through the logging middleware.
// Without this method Gorilla WebSocket cannot obtain the underlying connection.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not support hijacking")
	}
	return hijacker.Hijack()
}

// Unwrap allows http.ResponseController to access the underlying writer.
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// ============================================================
// СТАТИКА (SvelteKit)
// ============================================================

// handleStatic отдаёт статику из StaticDir.
// Если файл не найден — отдаёт index.html (для SPA-роутинга).
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	// Защита: не отдаём файлы вне StaticDir.
	cleanPath := filepath.Clean(r.URL.Path)
	if strings.Contains(cleanPath, "..") {
		http.NotFound(w, r)
		return
	}

	fullPath := filepath.Join(s.cfg.StaticDir, cleanPath)
	if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
		// Файл существует — отдаём его.
		http.ServeFile(w, r, fullPath)
		return
	}

	// Для /api/* или /admin/* — 404, если не найден.
	if strings.HasPrefix(r.URL.Path, "/api/") ||
		strings.HasPrefix(r.URL.Path, "/admin/") ||
		r.URL.Path == "/ws" {
		http.NotFound(w, r)
		return
	}

	// Иначе — отдаём index.html (SvelteKit сам разберётся).
	indexPath := filepath.Join(s.cfg.StaticDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		http.Error(w, "UI not built. Run: cd frontend && npm run build", http.StatusServiceUnavailable)
		return
	}
	http.ServeFile(w, r, indexPath)
}

// ============================================================
// HEALTH CHECK
// ============================================================

// Health возвращает статус сервера (для мониторинга).
func (s *Server) Health() map[string]interface{} {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	dbOK := "ok"
	if err := s.db.Ping(ctx); err != nil {
		dbOK = "error: " + err.Error()
	}

	return map[string]interface{}{
		"db":          dbOK,
		"online":      len(s.hub.List()),
		"admin":       s.adminUsername,
	}
}
