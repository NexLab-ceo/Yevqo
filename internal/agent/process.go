// Package agent — часть VELTRIX Dock, которая работает на телефонах пользователей.
// Этот файл: запуск/остановка процессов (php -S, Go, статика).
package agent

import (
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"veltrix/internal/shared"
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
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') { return false }
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
		if resolvedDataRoot, de := filepath.EvalSymlinks(dataRoot); de == nil {
			rootRel, re := filepath.Rel(resolvedDataRoot, resolvedRoot)
			if re != nil || rootRel == ".." || strings.HasPrefix(rootRel, ".."+string(filepath.Separator)) { return "", fmt.Errorf("sites directory resolves outside the agent data directory") }
		}
		if resolvedTarget, e := filepath.EvalSymlinks(absTarget); e == nil {
			rel, e := filepath.Rel(resolvedRoot, resolvedTarget)
			if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) { return "", fmt.Errorf("site path resolves outside the sites directory") }
		}
	}
	return absTarget, nil
}

func (m *ProcessManager) CreateSite(siteID string, port int, runtime string) error {
	if !validSiteID(siteID) { return fmt.Errorf("invalid site id") }
	if port < 1024 || port > 65535 { return fmt.Errorf("port must be between 1024 and 65535") }
	if runtime != "php" && runtime != "go" && runtime != "static" { return fmt.Errorf("unsupported runtime: %s", runtime) }
	target, err := safeSiteDirectory(m.DataRoot, siteID)
	if err != nil { return err }
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil { return fmt.Errorf("create sites directory: %w", err) }
	if err := os.Mkdir(target, 0o750); err != nil {
		if os.IsExist(err) { return fmt.Errorf("site directory already exists") }
		return fmt.Errorf("create site directory: %w", err)
	}
	fileName, body := "index.php", "<?php\n// Your VELTRIX website starts here.\necho 'Hello from VELTRIX';\n"
	if runtime == "static" { fileName, body = "index.html", "<!doctype html>\n<html lang=\"en\"><meta charset=\"utf-8\"><title>VELTRIX site</title><h1>Your site is ready</h1></html>\n" }
	if runtime == "go" { fileName, body = "README.md", "# Go site\n\nBuild your application as ./site before starting this site.\n" }
	f, err := os.OpenFile(filepath.Join(target, fileName), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil { _ = os.RemoveAll(target); return fmt.Errorf("create starter file: %w", err) }
	if _, err := f.WriteString(body); err != nil { f.Close(); _ = os.RemoveAll(target); return fmt.Errorf("write starter file: %w", err) }
	if err := f.Close(); err != nil { _ = os.RemoveAll(target); return fmt.Errorf("close starter file: %w", err) }
	return nil
}

func (m *ProcessManager) StartSite(siteID, path string, port int, runtime string) (int, error) {
	return m.StartSiteWithEnv(siteID, path, port, runtime, nil)
}

func (m *ProcessManager) StartSiteWithEnv(siteID, path string, port int, runtime string, extraEnv map[string]string) (int, error) {
	if !validSiteID(siteID) { return 0, fmt.Errorf("invalid site id") }
	if port < 1024 || port > 65535 { return 0, fmt.Errorf("port must be between 1024 and 65535") }
	if runtime != "php" && runtime != "go" && runtime != "static" { return 0, fmt.Errorf("unsupported runtime: %s", runtime) }
	cleanPath := filepath.Clean(path)
	expectedPath := filepath.Join("sites", siteID)
	if filepath.IsAbs(path) || (cleanPath != expectedPath && !strings.HasPrefix(cleanPath, expectedPath+string(filepath.Separator))) {
		return 0, fmt.Errorf("site path must be inside sites/<site_id>")
	}
	// Проверяем, что процесс ещё не запущен.
	if p, ok := m.procs[siteID]; ok && isAlive(p.PID) {
		return p.PID, fmt.Errorf("site %s is already running (pid %d)", siteID, p.PID)
	}

	// Resolve the requested root directory strictly inside this site's directory.
	siteRoot, err := safeSiteDirectory(m.DataRoot, siteID)
	if err != nil { return 0, err }
	relativeRoot := strings.TrimPrefix(cleanPath, expectedPath)
	relativeRoot = strings.TrimPrefix(relativeRoot, string(filepath.Separator))
	absPath := siteRoot
	if relativeRoot != "" { absPath = filepath.Join(siteRoot, relativeRoot) }
	resolvedSite, err := filepath.EvalSymlinks(siteRoot)
	if err != nil { return 0, fmt.Errorf("resolve site directory: %w", err) }
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil { return 0, fmt.Errorf("site root directory not found: %w", err) }
	rel, err := filepath.Rel(resolvedSite, resolvedPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return 0, fmt.Errorf("site root directory resolves outside the site")
	}

	// Refuse a port already occupied by another process.
	listener, listenErr := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if listenErr != nil { return 0, fmt.Errorf("port %d is already in use", port) }
	_ = listener.Close()

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
		cmd = exec.Command("php", "-S", fmt.Sprintf("127.0.0.1:%d", port), "-t", ".")
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
		cmd = exec.Command("php", "-S", fmt.Sprintf("127.0.0.1:%d", port), "-t", ".")
		cmd.Dir = absPath
	default:
		logFile.Close()
		return 0, fmt.Errorf("unsupported runtime: %s", runtime)
	}

	// Environment values are injected into this site's process only; no .env file is modified.
	env := append([]string{}, os.Environ()...)
	for key, value := range extraEnv {
		if key != "DB_CONNECTION" && key != "DB_HOST" && key != "DB_PORT" && key != "DB_SSLMODE" && key != "DB_DATABASE" && key != "DB_USERNAME" && key != "DB_PASSWORD" && key != "VELTRIX_DATABASES" {
			logFile.Close()
			return 0, fmt.Errorf("unsupported managed environment key: %s", key)
		}
		if strings.IndexByte(value, 0) >= 0 || len(value) > 16384 {
			logFile.Close()
			return 0, fmt.Errorf("invalid managed environment value")
		}
		env = append(env, key+"="+value)
	}
	if runtime == "go" { env = append(env, fmt.Sprintf("PORT=%d", port)) }
	cmd.Env = env

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
// RestartSite restarts a site without pulling source changes.
func (m *ProcessManager) RestartSite(siteID, path string, port int, runtime string) (int, error) {
	if err := m.StopSite(siteID); err != nil && !strings.Contains(err.Error(), "is not running") {
		return 0, err
	}
	return m.StartSite(siteID, path, port, runtime)
}

// DeploySite pulls the fast-forwardable Git revision, optionally rebuilds Go,
// then performs a stop/start cycle. Git conflicts and build errors are returned.
func (m *ProcessManager) DeploySite(siteID, path string, port int, runtime string, rebuild bool) (int, string, error) {
	if !validSiteID(siteID) { return 0, "", fmt.Errorf("invalid site id") }
	if filepath.Clean(path) != filepath.Join("sites", siteID) || filepath.IsAbs(path) {
		return 0, "", fmt.Errorf("site path must be sites/<site_id>")
	}
	absPath, err := safeSiteDirectory(m.DataRoot, siteID)
	if err != nil { return 0, "", err }
	if _, err := os.Stat(absPath); err != nil { return 0, "", fmt.Errorf("site directory not found: %w", err) }
	out, err := runGitPull(absPath)
	if err != nil { return 0, out, err }
	if rebuild && runtime == "go" {
		cmd := exec.Command("go", "build", "-o", "site", ".")
		cmd.Dir = absPath
		buildOut, buildErr := cmd.CombinedOutput()
		out += string(buildOut)
		if buildErr != nil { return 0, out, fmt.Errorf("go build: %w", buildErr) }
	}
	pid, err := m.RestartSite(siteID, path, port, runtime)
	return pid, out, err
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


// DeployGitSite clones or fast-forwards the configured repository, runs the optional
// build command in the configured root directory, and restarts the site there.
func (m *ProcessManager) DeployGitSite(req shared.SiteAction, rebuild bool) (int, string, string, string, error) {
	if !validSiteID(req.SiteId) { return 0, "", "", "", fmt.Errorf("invalid site id") }
	siteRoot, err := safeSiteDirectory(m.DataRoot, req.SiteId)
	if err != nil { return 0, "", "", "", err }
	if req.GitURL == "" { return 0, "", "", "", fmt.Errorf("Git repository is not configured") }
	branch := req.GitBranch
	if branch == "" { branch = "main" }
	if strings.ContainsAny(branch, " \t\r\n") || strings.Contains(branch, "..") || strings.HasPrefix(branch, "-") {
		return 0, "", "", "", fmt.Errorf("invalid Git branch")
	}
	gitEnv := append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if req.GitToken != "" {
		encoded := base64.StdEncoding.EncodeToString([]byte("x-access-token:"+req.GitToken))
		gitEnv = append(gitEnv, "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=http.extraheader",
			"GIT_CONFIG_VALUE_0=AUTHORIZATION: basic "+encoded)
	}
	var output strings.Builder
	if _, err := os.Stat(filepath.Join(siteRoot, ".git")); os.IsNotExist(err) {
		entries, readErr := os.ReadDir(siteRoot)
		if readErr != nil { return 0, "", "", "", readErr }
		for _, entry := range entries {
			name := entry.Name()
			if name != "index.php" && name != "index.html" && name != "README.md" {
				return 0, "", "", "", fmt.Errorf("site directory is not empty; refusing to overwrite existing files")
			}
		}
		for _, entry := range entries {
			if err := os.Remove(filepath.Join(siteRoot, entry.Name())); err != nil { return 0, "", "", "", err }
		}
		cmd := exec.Command("git", "clone", "--branch", branch, "--single-branch", req.GitURL, ".")
		cmd.Dir, cmd.Env = siteRoot, gitEnv
		out, cloneErr := cmd.CombinedOutput()
		output.Write(out)
		if cloneErr != nil { return 0, output.String(), "", "", fmt.Errorf("git clone: %w", cloneErr) }
	} else if err != nil {
		return 0, "", "", "", err
	} else {
		cmd := exec.Command("git", "-C", siteRoot, "pull", "--ff-only", "origin", branch)
		cmd.Env = gitEnv
		out, pullErr := cmd.CombinedOutput()
		output.Write(out)
		if pullErr != nil { return 0, output.String(), "", "", fmt.Errorf("git pull: %w", pullErr) }
	}
	rootDir := req.RootDir
	if rootDir == "" { rootDir = "." }
	if filepath.IsAbs(rootDir) || strings.Contains(rootDir, "\\") {
		return 0, output.String(), "", "", fmt.Errorf("invalid root directory")
	}
	rootDir = filepath.Clean(rootDir)
	if rootDir == ".." || strings.HasPrefix(rootDir, ".."+string(filepath.Separator)) {
		return 0, output.String(), "", "", fmt.Errorf("root directory must stay inside the site")
	}
	buildDir := siteRoot
	if rootDir != "." { buildDir = filepath.Join(siteRoot, rootDir) }
	resolvedRoot, resolveErr := filepath.EvalSymlinks(siteRoot)
	if resolveErr != nil { return 0, output.String(), "", "", resolveErr }
	resolvedBuild, resolveErr := filepath.EvalSymlinks(buildDir)
	if resolveErr != nil { return 0, output.String(), "", "", fmt.Errorf("root directory not found: %w", resolveErr) }
	if rel, relErr := filepath.Rel(resolvedRoot, resolvedBuild); relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return 0, output.String(), "", "", fmt.Errorf("root directory resolves outside the site")
	}
	if strings.TrimSpace(req.BuildCommand) != "" {
		cmd := exec.Command("sh", "-lc", req.BuildCommand)
		cmd.Dir, cmd.Env = resolvedBuild, os.Environ()
		out, buildErr := cmd.CombinedOutput()
		output.Write(out)
		if buildErr != nil { return 0, output.String(), "", "", fmt.Errorf("build command failed: %w", buildErr) }
	}
	commitCmd := exec.Command("git", "-C", siteRoot, "rev-parse", "HEAD")
	commitBytes, err := commitCmd.Output()
	if err != nil { return 0, output.String(), "", "", fmt.Errorf("read deployed commit: %w", err) }
	commit := strings.TrimSpace(string(commitBytes))
	dateCmd := exec.Command("git", "-C", siteRoot, "show", "-s", "--format=%cI", "HEAD")
	dateBytes, err := dateCmd.Output()
	if err != nil { return 0, output.String(), "", "", fmt.Errorf("read commit date: %w", err) }
	commitDate := strings.TrimSpace(string(dateBytes))
	runPath := filepath.Join("sites", req.SiteId)
	if rootDir != "." { runPath = filepath.Join(runPath, rootDir) }
	pid, err := m.RestartSite(req.SiteId, runPath, int(req.Port), req.Runtime)
	if err != nil { return 0, output.String(), commit, commitDate, err }
	return pid, output.String(), commit, commitDate, nil
}
