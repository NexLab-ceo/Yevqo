// Package agent — часть VELTRIX Dock, которая работает на телефонах пользователей.
// Этот файл: запуск/остановка Cloudflare Tunnel для сайтов.
package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// TunnelsDir — папка для конфигов cloudflared.
const tunnelsDir = "cloudflared"

// Tunnel — запущенный туннель для одного сайта.
type Tunnel struct {
	SiteID   string
	Hostname string
	PID      int
	Cmd      *exec.Cmd
	LogPath  string
}

// TunnelManager — менеджер туннелей.
type TunnelManager struct {
	DataRoot string
	tunnels  map[string]*Tunnel // key = siteID
}

// NewTunnelManager создаёт менеджер.
func NewTunnelManager(dataRoot string) *TunnelManager {
	return &TunnelManager{
		DataRoot: dataRoot,
		tunnels:  make(map[string]*Tunnel),
	}
}

// ============================================================
// START
// ============================================================

// StartTunnel запускает Cloudflare Tunnel для сайта.
//
// Параметры:
//   - siteID:        "blog"
//   - port:          локальный порт сайта (например, 8000)
//   - tunnelToken:   токен Cloudflare Tunnel (из панели Cloudflare)
//   - hostname:      публичный домен (например, "blog.user1.com")
func (m *TunnelManager) StartTunnel(siteID string, port int, tunnelToken, hostname string) (int, error) {
	// Проверяем, что cloudflared установлен.
	if _, err := exec.LookPath("cloudflared"); err != nil {
		return 0, errors.New("cloudflared not found (install it first)")
	}

	// Проверяем, что туннель ещё не запущен.
	if t, ok := m.tunnels[siteID]; ok && isAlive(t.PID) {
		return t.PID, fmt.Errorf("tunnel for %s is already running (pid %d)", siteID, t.PID)
	}

	// Папка для конфигов cloudflared.
	cfgDir := filepath.Join(m.DataRoot, tunnelsDir)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return 0, fmt.Errorf("create cloudflared dir: %w", err)
	}

	// Лог-файл для туннеля.
	logDir := filepath.Join(m.DataRoot, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return 0, fmt.Errorf("create logs dir: %w", err)
	}
	logPath := filepath.Join(logDir, siteID+"-tunnel.log")

	// Открываем лог.
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open tunnel log: %w", err)
	}

	// Формируем команду cloudflared.
	// Используем `cloudflared tunnel run --token <TOKEN>` — самый простой режим.
	// В этом режиме cloudflared подключается к Cloudflare и сам знает,
	// какой домен за ним закреплён (через панель Cloudflare).
	//
	// Ограничение: один токен = один туннель = один домен.
	// Если нужно несколько сайтов через один туннель — нужен config.yml.
	cmd := exec.Command("cloudflared", "tunnel", "run", "--no-autoupdate", "--token", tunnelToken)
	cmd.Dir = m.DataRoot
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Отвязываем от родителя.
	cmd.SysProcAttr = procAttr()

	// Запускаем.
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return 0, fmt.Errorf("start cloudflared: %w", err)
	}

	// Отвязываем от родителя.
	_ = cmd.Process.Release()

	// Ждём, пока cloudflared подключится к Cloudflare (2 секунды).
	time.Sleep(2 * time.Second)

	// Проверяем, что процесс жив.
	pid := cmd.Process.Pid
	if !isAlive(pid) {
		logFile.Close()
		return 0, fmt.Errorf("cloudflared died immediately (check %s)", logPath)
	}

	// Сохраняем.
	m.tunnels[siteID] = &Tunnel{
		SiteID:   siteID,
		Hostname: hostname,
		PID:      pid,
		Cmd:      cmd,
		LogPath:  logPath,
	}

	log.Printf("tunnel started for %s: %s -> localhost:%d (pid %d)",
		siteID, hostname, port, pid)

	return pid, nil
}

// ============================================================
// STOP
// ============================================================

// StopTunnel останавливает туннель сайта.
func (m *TunnelManager) StopTunnel(siteID string) error {
	t, ok := m.tunnels[siteID]
	if !ok {
		// Попробуем найти процесс по siteID через pgrep.
		pid := findTunnelPID(siteID)
		if pid == 0 {
			return fmt.Errorf("tunnel for %s is not running", siteID)
		}
		return killProcess(pid)
	}

	if err := killProcess(t.PID); err != nil {
		return err
	}

	delete(m.tunnels, siteID)
	return nil
}

// ============================================================
// STATUS
// ============================================================

// IsRunning проверяет, запущен ли туннель для сайта.
func (m *TunnelManager) IsRunning(siteID string) bool {
	t, ok := m.tunnels[siteID]
	return ok && isAlive(t.PID)
}

// ListRunning — все активные туннели.
func (m *TunnelManager) ListRunning() []*Tunnel {
	out := make([]*Tunnel, 0, len(m.tunnels))
	for _, t := range m.tunnels {
		if isAlive(t.PID) {
			out = append(out, t)
		}
	}
	return out
}

// ============================================================
// CONFIG (config.yml для нескольких сайтов через один туннель)
// ============================================================

// TunnelConfig — описание одного ingress-правила.
type TunnelConfig struct {
	Hostname string
	Service  string // "http://localhost:8000"
}

// WriteTunnelConfig пишет config.yml для cloudflared.
// Используется, если пользователь хочет один туннель для нескольких сайтов.
//
// Пример:
//
//	tunnel: <UUID>
//	credentials-file: /path/<UUID>.json
//	ingress:
//	  - hostname: blog.user1.com
//	    service: http://localhost:8000
//	  - hostname: shop.user1.com
//	    service: http://localhost:8001
//	  - service: http_status:404
func (m *TunnelManager) WriteTunnelConfig(tunnelUUID, credentialsPath string, rules []TunnelConfig) (string, error) {
	cfgDir := filepath.Join(m.DataRoot, tunnelsDir)
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return "", fmt.Errorf("create dir: %w", err)
	}

	cfgPath := filepath.Join(cfgDir, "config.yml")

	var sb strings.Builder
	sb.WriteString("tunnel: " + tunnelUUID + "\n")
	sb.WriteString("credentials-file: " + credentialsPath + "\n")
	sb.WriteString("ingress:\n")
	for _, r := range rules {
		sb.WriteString("  - hostname: " + r.Hostname + "\n")
		sb.WriteString("    service: " + r.Service + "\n")
	}
	sb.WriteString("  - service: http_status:404\n")

	if err := os.WriteFile(cfgPath, []byte(sb.String()), 0o644); err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}

	return cfgPath, nil
}

// ============================================================
// УТИЛИТЫ
// ============================================================

// findTunnelPID ищет PID cloudflared по siteID (если агент перезапускался).
func findTunnelPID(siteID string) int {
	// Ищем cloudflared, у которого в аргументах есть "--token" и siteID
	// (или имя туннеля).
	cmd := exec.Command("pgrep", "-f", "cloudflared.*"+siteID)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if pid, err := parsePID(line); err == nil && pid > 0 {
			return pid
		}
	}
	return 0
}

// CheckCloudflared проверяет, установлен ли cloudflared.
func CheckCloudflared() (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "cloudflared", "--version")
	if err := cmd.Run(); err != nil {
		return false, "cloudflared not found (install it first)"
	}
	return true, "ok"
}
