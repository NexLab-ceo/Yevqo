// Package agent — часть VELTRIX Dock, которая работает на телефонах пользователей.
// Этот файл: запуск/остановка процессов (php -S, Go, статика).
package agent

import (
	"fmt"
	"os"
	"os/exec"
	"unicode"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Process — запущенный процесс сайта.
type Process struct {
	SiteID  string
	PID     int
	Port    int
	Runtime string
	LogPath string
	Cmd     *exec.Cmd
}

// ProcessManager — менеджер запущенных процессов.
type ProcessManager struct {
	DataRoot string
	procs    map[string]*Process // key = siteID
}

// NewProcessManager создаёт менеджер.
func NewProcessManager(dataRoot string) *ProcessManager {
	return &ProcessManager{
		DataRoot: dataRoot,
		procs:    make(map[string]*Process),
	}
}

// ============================================================
// START
// ============================================================

// StartSite запускает сайт и возвращает PID.
//
// Параметры:
//   - siteID:  "blog"
//   - path:    "sites/blog" (относительно DataRoot)
//   - port:    8000
//   - runtime: "php", "go", "static"
func validSiteID(siteID string) bool {
	if len(siteID) < 1 || len(siteID) > 64 { return false }
	for _, r := range siteID {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_') { return false }
	}
	return true
}

func safeSiteDirectory(dataRoot, siteID string) (string, error) {
	if !validSiteID(siteID) { return "", fmt.Errorf("invalid site id") }
	root := filepath.Join(dataRoot, "sites")
	target := filepath.Join(root, siteID)
	absRoot, err := filepath.Abs(root); if err != nil { return "", err }
	absTarget, err := filepath.Abs(target); if err != nil { return "", err }
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("site path is outside the sites directory")
	}
	if resolvedRoot, e := filepath.EvalSymlinks(absRoot); e == nil {
		if resolvedTarget, e := filepath.EvalSymlinks(absTarget); e == nil {
			rel, e = filepath.Rel(resolvedRoot, resolvedTarget)
			if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) { return "", fmt.Errorf("site path resolves outside the sites directory") }
		}
	}
	return absTarget, nil
}

func (m *ProcessManager) StartSite(siteID, path string, port int, runtime string) (int, error) {
	if !validSiteID(siteID) { return 0, fmt.Errorf("invalid site id") }
	if port < 1024 || port > 65535 { return 0, fmt.Errorf("port must be between 1024 and 65535") }
	if runtime != "php" && runtime != "go" && runtime != "static" { return 0, fmt.Errorf("unsupported runtime: %s", runtime) }
	if filepath.Clean(path) != filepath.Join("sites", siteID) || filepath.IsAbs(path) { return 0, fmt.Errorf("site path must be sites/<site_id>") }
	// Проверяем, что процесс ещё не запущен.
	if p, ok := m.procs[siteID]; ok && isAlive(p.PID) {
		return p.PID, fmt.Errorf("site %s is already running (pid %d)", siteID, p.PID)
	}

	// Resolve the site strictly inside the agent's sites directory.
	absPath, err := safeSiteDirectory(m.DataRoot, siteID)
	if err != nil { return 0, err }
	if _, err := os.Stat(absPath); err != nil {
		return 0, fmt.Errorf("site directory not found: %s", absPath)
	}

	// Лог-файл.
	logDir := filepath.Join(m.DataRoot, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return 0, fmt.Errorf("create logs dir: %w", err)
	}
	logPath := filepath.Join(logDir, siteID+".log")

	// Открываем лог для записи (append).
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return 0, fmt.Errorf("open log: %w", err)
	}

	// Формируем команду в зависимости от runtime.
	var cmd *exec.Cmd
	switch runtime {
	case "php":
		cmd = exec.Command("php", "-S", fmt.Sprintf("0.0.0.0:%d", port), "-t", ".")
		cmd.Dir = absPath
	case "go":
		// Для Go предполагаем, что бинарник уже собран в папке сайта.
		binary := filepath.Join(absPath, "site")
		if _, err := os.Stat(binary); err != nil {
			logFile.Close()
			return 0, fmt.Errorf("go binary not found: %s (build it first)", binary)
		}
		cmd = exec.Command(binary)
		cmd.Dir = absPath
		cmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port))
	case "static":
		// Статика — через php -S (он умеет отдавать статические файлы).
		cmd = exec.Command("php", "-S", fmt.Sprintf("0.0.0.0:%d", port), "-t", ".")
		cmd.Dir = absPath
	default:
		logFile.Close()
		return 0, fmt.Errorf("unsupported runtime: %s", runtime)
	}

	// Перенаправляем вывод в лог.
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Отвязываем от родителя (чтобы не умер при рестарте агента).
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // своя process group
	}

	// Запускаем.
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return 0, fmt.Errorf("start %s: %w", runtime, err)
	}

	// Отвязываем от родителя (чтобы не ждать завершения).
	if err := cmd.Process.Release(); err != nil {
		// Не критично, но логируем.
		fmt.Fprintf(logFile, "warning: process release: %v\n", err)
	}

	// Ждём, пока процесс займёт порт (200 мс).
	time.Sleep(200 * time.Millisecond)

	// Проверяем, что процесс жив.
	pid := cmd.Process.Pid
	if !isAlive(pid) {
		logFile.Close()
		return 0, fmt.Errorf("process died immediately (check %s)", logPath)
	}

	// Сохраняем в память.
	m.procs[siteID] = &Process{
		SiteID:  siteID,
		PID:     pid,
		Port:    port,
		Runtime: runtime,
		LogPath: logPath,
		Cmd:     cmd,
	}

	return pid, nil
}

// ============================================================
// STOP
// ============================================================

// StopSite останавливает сайт (SIGTERM, потом SIGKILL).
func (m *ProcessManager) StopSite(siteID string) error {
	if !validSiteID(siteID) { return fmt.Errorf("invalid site id") }
	p, ok := m.procs[siteID]
	if !ok {
		// Попробуем найти PID через pgrep (на случай, если агент перезапускался).
		pid := findPIDBySite(m.DataRoot, siteID)
		if pid == 0 {
			return fmt.Errorf("site %s is not running", siteID)
		}
		// Убиваем.
		if err := killProcess(pid); err != nil {
			return err
		}
		return nil
	}

	// Есть в памяти — убиваем.
	if err := killProcess(p.PID); err != nil {
		return err
	}

	delete(m.procs, siteID)
	return nil
}

// ============================================================
// LOGS
// ============================================================

// GetLogs возвращает последние N строк лога сайта.
func (m *ProcessManager) GetLogs(siteID string, lines int) (string, error) {
	if !validSiteID(siteID) { return "", fmt.Errorf("invalid site id") }
	logPath := filepath.Join(m.DataRoot, "logs", siteID+".log")
	if _, err := os.Stat(logPath); err != nil {
		return "", fmt.Errorf("log not found: %s", logPath)
	}

	if lines <= 0 {
		lines = 50
	}

	cmd := exec.Command("tail", "-n", strconv.Itoa(lines), logPath)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tail: %w", err)
	}

	return string(out), nil
}

// ============================================================
// УТИЛИТЫ
// ============================================================

// isAlive проверяет, жив ли процесс.
func isAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	// signal 0 = "проверка живости"
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// killProcess убивает процесс (SIGTERM, потом SIGKILL через 2 сек).
func killProcess(pid int) error {
	if !isAlive(pid) {
		return fmt.Errorf("process %d is not running", pid)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process: %w", err)
	}

	// Сначала SIGTERM (вежливо).
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("signal SIGTERM: %w", err)
	}

	// Ждём 2 секунды.
	time.Sleep(2 * time.Second)

	// Если всё ещё жив — SIGKILL.
	if isAlive(pid) {
		if err := proc.Signal(syscall.SIGKILL); err != nil {
			return fmt.Errorf("signal SIGKILL: %w", err)
		}
	}

	return nil
}

// findPIDBySite ищет PID процесса по сайту через pgrep.
// Используется, если агент перезапускался и не знает свой PID.
//
// Ищет процессы, у которых в командной строке есть "sites/<siteID>"
// или "php -S 0.0.0.0:<port>".
func findPIDBySite(dataRoot, siteID string) int {
	if !validSiteID(siteID) { return 0 }
	// Ищем php -S, который был запущен из папки этого сайта.
	pattern := filepath.Join(dataRoot, "sites", siteID)
	cmd := exec.Command("pgrep", "-f", pattern)
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
		if pid, err := strconv.Atoi(line); err == nil && pid > 0 {
			return pid
		}
	}
	return 0
}

// ============================================================
// ОБЩИЕ МЕТОДЫ
// ============================================================

// ListRunning возвращает список запущенных сайтов.
func (m *ProcessManager) ListRunning() []*Process {
	out := make([]*Process, 0, len(m.procs))
	for _, p := range m.procs {
		if isAlive(p.PID) {
			out = append(out, p)
		}
	}
	return out
}

// Get возвращает процесс по siteID.
func (m *ProcessManager) Get(siteID string) (*Process, bool) {
	p, ok := m.procs[siteID]
	return p, ok
}

// StopAll останавливает все процессы (при выключении агента).
func (m *ProcessManager) StopAll() {
	for siteID := range m.procs {
		_ = m.StopSite(siteID)
	}
}
// procAttr возвращает SysProcAttr для отвязки процесса.
func procAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

// parsePID парсит строку в int.
func parsePID(s string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(s))
}
// runGitPull выполняет git pull в папке сайта и возвращает вывод.
func runGitPull(path string) (string, error) {
	// Проверяем, что это git-репозиторий.
	if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
		return "", fmt.Errorf("not a git repository: %s", path)
	}

	cmd := exec.Command("git", "pull", "--ff-only")
	cmd.Dir = path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git pull: %w", err)
	}

	return string(out), nil
}
