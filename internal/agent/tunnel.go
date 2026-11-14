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
	"runtime"
	"net/http"
	"io"
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
	if !validTunnelSiteID(siteID) { return 0, errors.New("invalid site id") }
	if port < 1024 || port > 65535 { return 0, errors.New("invalid site port") }
	if strings.TrimSpace(tunnelToken) == "" { return 0, errors.New("Cloudflare Tunnel token is required") }
	if strings.TrimSpace(hostname) == "" || strings.ContainsAny(hostname, "/\\\\ \r\n\t") { return 0, errors.New("invalid public hostname") }
	// Проверяем, что cloudflared установлен.
	binary, err := exec.LookPath("cloudflared")
	if err != nil {
		binary = filepath.Join(m.DataRoot, "bin", "cloudflared")
		if _, statErr := os.Stat(binary); statErr != nil {
			return 0, errors.New("cloudflared not found (install it first)")
		}
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
	tokenPath := filepath.Join(cfgDir, siteID+".token")
	if err := os.WriteFile(tokenPath, []byte(strings.TrimSpace(tunnelToken)+"\n"), 0o600); err != nil { return 0, fmt.Errorf("store tunnel token: %w", err) }
	if err := os.Chmod(tokenPath, 0o600); err != nil { return 0, fmt.Errorf("secure tunnel token: %w", err) }

	// Логи и токен доступны только пользователю Agent.
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return 0, fmt.Errorf("open tunnel log: %w", err)
	}

	// Не передаём секрет в argv: его можно было увидеть через список процессов.
	// Для remotely-managed Tunnel Cloudflare хранит ingress-маршруты в Zero Trust.
	cmd := exec.Command(binary, "tunnel", "run", "--no-autoupdate", "--token-file", tokenPath)
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

	// Родительский процесс закрывает свой дескриптор; дочерний сохраняет stdout/stderr.
	_ = logFile.Close()
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()

	// Ждём подтверждения регистрации соединения на edge, а не просто живого PID.
	if err := waitForTunnelReady(pid, logPath, 20*time.Second); err != nil {
		_ = killProcess(pid)
		return 0, fmt.Errorf("Cloudflare Tunnel did not become ready: %w (see %s)", err, logPath)
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


// validTunnelSiteID keeps site identifiers safe for token/log filenames and pgrep patterns.
func validTunnelSiteID(id string) bool {
 if id == "" || len(id) > 64 { return false }
 for _, r := range id { if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '-' && r != '_' { return false } }
 return true
}

// waitForTunnelReady requires cloudflared to report an edge connection registration.
func waitForTunnelReady(pid int, logPath string, timeout time.Duration) error {
 deadline := time.Now().Add(timeout)
 for time.Now().Before(deadline) {
  if !isAlive(pid) { return errors.New("cloudflared process exited before registering an edge connection") }
  data, err := os.ReadFile(logPath)
  if err == nil {
   logs := strings.ToLower(string(data))
   if strings.Contains(logs, "registered tunnel connection") || strings.Contains(logs, "connection registered") { return nil }
   if strings.Contains(logs, "invalid tunnel token") || strings.Contains(logs, "unauthorized") { return errors.New("Cloudflare rejected the tunnel credentials") }
  }
  time.Sleep(500 * time.Millisecond)
 }
 return errors.New("timed out waiting for a registered tunnel connection")
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
	_ = os.Remove(filepath.Join(m.DataRoot, tunnelsDir, siteID+".token"))
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
	cmd := exec.Command("pgrep", "-f", "cloudflared.*"+siteID+"\\.token")
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

 
// InstallCloudflared downloads the official Linux release into the user's VELTRIX directory.
// Android/Termux runs without root; the binary is installed in DataRoot/bin.
func (m *TunnelManager) InstallCloudflared() (string, error) {
 arch := runtime.GOARCH
 if arch != "arm64" && arch != "amd64" { return "", fmt.Errorf("unsupported architecture: %s", arch) }
 binDir := filepath.Join(m.DataRoot, "bin")
 if err := os.MkdirAll(binDir, 0o755); err != nil { return "", err }
 target := filepath.Join(binDir, "cloudflared")
 req, err := http.NewRequest(http.MethodGet, "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-"+arch, nil)
 if err != nil { return "", err }
 ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute); defer cancel(); req=req.WithContext(ctx)
 resp, err := (&http.Client{Timeout:3*time.Minute}).Do(req); if err != nil { return "", fmt.Errorf("download cloudflared: %w",err) }; defer resp.Body.Close()
 if resp.StatusCode != http.StatusOK { return "", fmt.Errorf("Cloudflare download returned HTTP %d",resp.StatusCode) }
 tmp:=target+".tmp"; file,err:=os.OpenFile(tmp,os.O_CREATE|os.O_TRUNC|os.O_WRONLY,0o700);if err!=nil{return "",err}
 n,copyErr:=io.Copy(file,io.LimitReader(resp.Body,100<<20));closeErr:=file.Close()
 if copyErr!=nil{_ = os.Remove(tmp);return "",copyErr};if closeErr!=nil{_ = os.Remove(tmp);return "",closeErr}
 if n<1<<20{_ = os.Remove(tmp);return "",errors.New("downloaded cloudflared binary is unexpectedly small")}
 if err=os.Chmod(tmp,0o755);err!=nil{_ = os.Remove(tmp);return "",err}
 if err=os.Rename(tmp,target);err!=nil{_ = os.Remove(tmp);return "",err}
 return target,nil
}
