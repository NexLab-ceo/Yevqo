// Package agent — часть Yevqo, которая работает на телефонах пользователей.
// Этот файл: запуск/остановка процессов (php -S, Go, статика).
package agent

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sort"
	"strings"
	"sync"
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
	// operationMu prevents lifecycle, build and release mutations from interleaving.
	operationMu sync.Mutex
	
	mu sync.RWMutex
	DataRoot string
	BuildRunner *AndroidBuildRunner
	registry *ProcessRegistry
	registryErr error
	logEmitter func(string, string)
	statusEmitter func(string, string, int)
	procs    map[string]*Process // key = siteID
	restartMu sync.Mutex
	restartAttempts map[string]int
	intentionalStops map[string]bool
	jobMu sync.Mutex
	deploymentJobs map[string]*deploymentJob
	deploymentEmitter func(shared.DeploymentJobEvent)
}

// NewProcessManager создаёт менеджер.
func NewProcessManager(dataRoot string) *ProcessManager {
 m := &ProcessManager{DataRoot:dataRoot, BuildRunner:NewAndroidBuildRunner(dataRoot), procs:make(map[string]*Process), restartAttempts:make(map[string]int), intentionalStops:make(map[string]bool)}
 m.registry, m.registryErr = NewProcessRegistry(dataRoot)
 if m.registryErr == nil { m.restoreProcesses(); go m.watchProcesses() }
 return m
}

// SetLogEmitter installs a non-blocking best-effort callback for process output.
// SetStatusEmitter reports lifecycle transitions to the Control Plane.
func (m *ProcessManager) SetStatusEmitter(emit func(siteID, status string, pid int)) {
	m.mu.Lock()
	m.statusEmitter = emit
	m.mu.Unlock()
}

func (m *ProcessManager) SetLogEmitter(emit func(siteID, line string)) {
	m.mu.Lock()
	m.logEmitter = emit
	processes := make([]*Process, 0, len(m.procs))
	for _, process := range m.procs {
		if isAlive(process.PID) { processes = append(processes, process) }
	}
	m.mu.Unlock()

	// Resume streaming for processes restored from the durable registry after
	// an Agent restart. Start at EOF to avoid replaying old log history.
	for _, process := range processes {
		info, err := os.Stat(process.LogPath)
		if err != nil { continue }
		go m.streamLogFile(process.SiteID, process.LogPath, info.Size(), process.PID, emit)
	}
}

func (m *ProcessManager) streamLogFile(siteID, logPath string, offset int64, pid int, emit func(string, string)) {
	file, err := os.Open(logPath)
	if err != nil { return }
	defer file.Close()
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	pending := make([]byte, 0, 4096)
	for {
		buffer := make([]byte, 4096)
		n, readErr := file.ReadAt(buffer, offset)
		if n > 0 {
			offset += int64(n)
			pending = append(pending, buffer[:n]...)
			for {
				i := strings.IndexByte(string(pending), '\n')
				if i < 0 { break }
				line := strings.TrimSuffix(string(pending[:i]), "\r")
				pending = append([]byte(nil), pending[i+1:]...)
				if emit != nil { emit(siteID, line) }
			}
			if len(pending) > 8192 {
				if emit != nil { emit(siteID, string(pending[:8192])) }
				pending = append([]byte(nil), pending[8192:]...)
			}
		}
		if readErr != nil && readErr != io.EOF { return }
		m.mu.RLock()
		process, managed := m.procs[siteID]
		stillManaged := managed && process.PID == pid
		m.mu.RUnlock()
		if !stillManaged || !isAlive(pid) {
			if len(pending) > 0 && emit != nil { emit(siteID, string(pending)) }
			return
		}
		<-ticker.C
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
	if info, statErr := os.Lstat(absRoot); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("sites directory must not be a symlink")
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return "", fmt.Errorf("inspect sites directory: %w", statErr)
	}
	if info, statErr := os.Lstat(absTarget); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("site directory must not be a symlink")
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return "", fmt.Errorf("inspect site directory: %w", statErr)
	}
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

// AnalyzeSiteBuild inspects a configured site root without executing project code.
func (m *ProcessManager) AnalyzeSiteBuild(siteID, rootDir string) (BuildPlan, error) {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	if !validSiteID(siteID) {
		return BuildPlan{}, fmt.Errorf("invalid site id")
	}
	siteRoot, err := safeSiteDirectory(m.DataRoot, siteID)
	if err != nil {
		return BuildPlan{}, err
	}
	if rootDir == "" {
		rootDir = "."
	}
	if filepath.IsAbs(rootDir) || strings.Contains(rootDir, "\\") {
		return BuildPlan{}, fmt.Errorf("root directory must stay inside the site")
	}
	clean := filepath.Clean(rootDir)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return BuildPlan{}, fmt.Errorf("root directory must stay inside the site")
	}
	target := siteRoot
	if clean != "." {
		target = filepath.Join(siteRoot, clean)
	}
	resolvedSite, err := filepath.EvalSymlinks(siteRoot)
	if err != nil {
		return BuildPlan{}, fmt.Errorf("site directory not found: %w", err)
	}
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		return BuildPlan{}, fmt.Errorf("project root not found: %w", err)
	}
	rel, err := filepath.Rel(resolvedSite, resolvedTarget)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return BuildPlan{}, fmt.Errorf("project root resolves outside the site")
	}
	plan, err := AnalyzeProject(resolvedTarget)
	if err != nil {
		return BuildPlan{}, err
	}
	plan.Root = clean
	return plan, nil
}

func (m *ProcessManager) CreateSite(siteID string, port int, runtime string) error {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	if !validSiteID(siteID) { return fmt.Errorf("invalid site id") }
	if port < 1024 || port > 65535 { return fmt.Errorf("port must be between 1024 and 65535") }
	if runtime != "php" && runtime != "go" && runtime != "static" && runtime != "nodejs" { return fmt.Errorf("unsupported runtime: %s", runtime) }
	target, err := safeSiteDirectory(m.DataRoot, siteID)
	if err != nil { return err }
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil { return fmt.Errorf("create sites directory: %w", err) }
	if err := os.Mkdir(target, 0o750); err != nil {
		if os.IsExist(err) { return fmt.Errorf("site directory already exists") }
		return fmt.Errorf("create site directory: %w", err)
	}
	fileName, body := "index.php", "<?php\n// Your Yevqo website starts here.\necho 'Hello from Yevqo';\n"
	if runtime == "static" { fileName, body = "index.html", "<!doctype html>\n<html lang=\"en\"><meta charset=\"utf-8\"><title>Yevqo site</title><h1>Your site is ready</h1></html>\n" }
	if runtime == "go" { fileName, body = "README.md", "# Go site\n\nBuild your application as ./site before starting this site.\n" }
	if runtime == "nodejs" {
		packageJSON := "{\n  \"name\": \"yevqo-site\",\n  \"version\": \"1.0.0\",\n  \"private\": true,\n  \"scripts\": { \"start\": \"node server.js\" }\n}\n"
		if err := os.WriteFile(filepath.Join(target, "package.json"), []byte(packageJSON), 0o640); err != nil { _ = os.RemoveAll(target); return fmt.Errorf("create Node.js package manifest: %w", err) }
		server := "const http = require('http');\nconst host = process.env.HOST || '127.0.0.1';\nconst port = Number(process.env.PORT || 3000);\nhttp.createServer((_req, res) => { res.writeHead(200, { 'content-type': 'text/plain; charset=utf-8' }); res.end('Hello from Yevqo'); }).listen(port, host);\n"
		if err := os.WriteFile(filepath.Join(target, "server.js"), []byte(server), 0o640); err != nil { _ = os.RemoveAll(target); return fmt.Errorf("create Node.js starter server: %w", err) }
		return nil
	}
	f, err := os.OpenFile(filepath.Join(target, fileName), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil { _ = os.RemoveAll(target); return fmt.Errorf("create starter file: %w", err) }
	if _, err := f.WriteString(body); err != nil { f.Close(); _ = os.RemoveAll(target); return fmt.Errorf("write starter file: %w", err) }
	if err := f.Close(); err != nil { _ = os.RemoveAll(target); return fmt.Errorf("close starter file: %w", err) }
	return nil
}

// nodeStartCommand selects a local Node package manager without invoking a shell.
func nodeStartCommand(root string) (*exec.Cmd, error) {
	for _, candidate := range []struct{ lock, bin string }{
		{"bun.lock", "bun"}, {"bun.lockb", "bun"},
		{"pnpm-lock.yaml", "pnpm"}, {"yarn.lock", "yarn"},
	} {
		if _, err := os.Stat(filepath.Join(root, candidate.lock)); err == nil {
			if _, err := exec.LookPath(candidate.bin); err != nil { return nil, fmt.Errorf("%s is required by this Node.js project but is not installed", candidate.bin) }
			return exec.Command(candidate.bin, "run", "start"), nil
		}
	}
	if _, err := exec.LookPath("npm"); err != nil { return nil, fmt.Errorf("npm is required to start this Node.js project but is not installed") }
	return exec.Command("npm", "run", "start"), nil
}

func (m *ProcessManager) StartSite(siteID, path string, port int, runtime string) (int, error) {
	return m.StartSiteWithEnv(siteID, path, port, runtime, nil)
}

func (m *ProcessManager) StartSiteWithEnv(siteID, path string, port int, runtime string, extraEnv map[string]string) (int, error) {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	return m.startSiteWithEnvLocked(siteID, path, port, runtime, extraEnv)
}

func (m *ProcessManager) startSiteWithEnvLocked(siteID, path string, port int, runtime string, extraEnv map[string]string) (int, error) {
	if !validSiteID(siteID) { return 0, fmt.Errorf("invalid site id") }
	if port < 1024 || port > 65535 { return 0, fmt.Errorf("port must be between 1024 and 65535") }
	if runtime != "php" && runtime != "go" && runtime != "static" && runtime != "nodejs" { return 0, fmt.Errorf("unsupported runtime: %s", runtime) }
	cleanPath := filepath.Clean(path)
	expectedPath := filepath.Join("sites", siteID)
	if filepath.IsAbs(path) || (cleanPath != expectedPath && !strings.HasPrefix(cleanPath, expectedPath+string(filepath.Separator))) {
		return 0, fmt.Errorf("site path must be inside sites/<site_id>")
	}
	// Проверяем, что процесс ещё не запущен.
	m.mu.RLock()
	p, alreadyManaged := m.procs[siteID]
	m.mu.RUnlock()
	if alreadyManaged && isAlive(p.PID) {
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

	// Rotate old output before opening the next process log.
	if err := rotateSiteLog(logPath); err != nil { return 0, err }

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
			// The built-in Go provider emits veltrix-app; retain compatibility
			// with existing manually supplied "site" binaries.
			binary = filepath.Join(absPath, "veltrix-app")
			if _, fallbackErr := os.Stat(binary); fallbackErr != nil {
				logFile.Close()
				return 0, fmt.Errorf("go binary not found: expected site or veltrix-app in %s (build it first)", absPath)
			}
		}
		cmd = exec.Command(binary)
		cmd.Dir = absPath
		cmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port))
	case "nodejs":
		cmd, err = nodeStartCommand(absPath)
		if err != nil { logFile.Close(); return 0, err }
		cmd.Dir = absPath
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
	if runtime == "go" || runtime == "nodejs" { env = append(env, fmt.Sprintf("PORT=%d", port)) }
	if runtime == "nodejs" { env = append(env, "HOST=127.0.0.1") }
	cmd.Env = env

	// Keep process output attached directly to the file. The Agent tails the
	// file separately so child processes never inherit a Go-managed output pipe.
	logInfo, _ := logFile.Stat()
	logStartOffset := int64(0)
	if logInfo != nil { logStartOffset = logInfo.Size() }
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

	// Wait for TCP readiness rather than assuming a fixed startup delay.
	pid := cmd.Process.Pid
	readinessTimeout := 5 * time.Second
	if runtime == "nodejs" { readinessTimeout = 20 * time.Second }
	if err := waitForTCP(port, readinessTimeout); err != nil || !isAlive(pid) {
		// A process that never becomes ready must not remain unmanaged.
		if isAlive(pid) {
			if stopErr := killProcessGroup(pid, pid); stopErr != nil {
				fmt.Fprintf(logFile, "failed to stop process after readiness failure: %v\\n", stopErr)
			}
		}
		go cmd.Wait() // Reap the child even when startup fails.
		_ = logFile.Close()
		if err != nil {
			return 0, fmt.Errorf("process failed readiness check (check %s): %w", logPath, err)
		}
		return 0, fmt.Errorf("process exited before becoming ready (check %s)", logPath)
	}

	// Persist verified process identity before exposing it as managed.
	startToken, identityErr := processStartToken(pid)
	if identityErr != nil {
		_ = killProcessGroup(pid, pid)
		go cmd.Wait()
		logFile.Close()
		return 0, fmt.Errorf("capture process identity: %w", identityErr)
	}
	if m.registryErr != nil {
		_ = killProcessGroup(pid, pid)
		go cmd.Wait()
		logFile.Close()
		return 0, fmt.Errorf("process registry unavailable: %w", m.registryErr)
	}
	restartPolicy, policyErr := m.GetRestartPolicy(siteID)
	if policyErr != nil { _ = killProcessGroup(pid, pid); go cmd.Wait(); _ = logFile.Close(); return 0, fmt.Errorf("load restart policy: %w", policyErr) }
	record := ProcessRecord{SiteID:siteID, PID:pid, ProcessGroup:pid, Port:port, Runtime:runtime, WorkingDir:absPath, LogPath:logPath, StartedAt:time.Now().UTC(), StartToken:startToken, RunPath:cleanPath, RestartPolicy:restartPolicy}
	if err := m.registry.Save(record); err != nil {
		_ = killProcessGroup(pid, pid)
		go cmd.Wait()
		logFile.Close()
		return 0, fmt.Errorf("persist process record: %w", err)
	}
	m.mu.Lock()
	m.procs[siteID] = &Process{
		SiteID:  siteID,
		PID:     pid,
		Port:    port,
		Runtime: runtime,
		LogPath: logPath,
		Cmd:     cmd,
	}
	m.mu.Unlock()

	// Keep a waiter attached while the Agent is alive so exited children are reaped.
	go m.waitForProcessExit(siteID, pid, startToken, cmd)
	m.mu.RLock()
	emitLog := m.logEmitter
	m.mu.RUnlock()
	go m.streamLogFile(siteID, logPath, logStartOffset, pid, emitLog)
	return pid, nil
}

// ============================================================
// STOP
// ============================================================

// StopSite останавливает сайт (SIGTERM, потом SIGKILL).
func (m *ProcessManager) StopSite(siteID string) error {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	return m.stopSiteLocked(siteID)
}

func (m *ProcessManager) stopSiteLocked(siteID string) error {
	if !validSiteID(siteID) { return fmt.Errorf("invalid site id") }
	if m.registryErr != nil || m.registry == nil { return fmt.Errorf("process registry unavailable: %v", m.registryErr) }
	record, exists := m.registry.Get(siteID)
	if !exists { return fmt.Errorf("site %s is not running", siteID) }
	if !verifyProcessRecord(record) { _ = m.registry.Delete(siteID); m.mu.Lock(); delete(m.procs,siteID); m.mu.Unlock(); return fmt.Errorf("site %s process identity is stale; refusing to signal PID",siteID) }
	m.restartMu.Lock(); m.intentionalStops[siteID] = true; m.restartMu.Unlock()
	if err := killProcessGroup(record.PID, record.ProcessGroup); err != nil { m.restartMu.Lock(); delete(m.intentionalStops, siteID); m.restartMu.Unlock(); return err }
	m.mu.Lock()
	delete(m.procs, siteID)
	m.mu.Unlock()
	err := m.registry.Delete(siteID)
	m.restartMu.Lock(); delete(m.intentionalStops, siteID); delete(m.restartAttempts, siteID); m.restartMu.Unlock()
	return err
}

// ============================================================
// LOGS
// ============================================================

// GetLogs возвращает последние N строк лога сайта.
func (m *ProcessManager) GetLogs(siteID string, lines int) (string, error) {
 return readSiteLogs(m.DataRoot, siteID, lines)
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
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Process, 0, len(m.procs))
	for _, p := range m.procs { if isAlive(p.PID) { out=append(out,p) } }
	return out
}

// Get возвращает процесс по siteID.
func (m *ProcessManager) Get(siteID string) (*Process, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.procs[siteID]
	return p, ok
}

// StopAll останавливает все процессы (при выключении агента).
func (m *ProcessManager) StopAll() {
	if m.registry == nil {
		return
	}
	for _, record := range m.registry.List() {
		_ = m.StopSite(record.SiteID)
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
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	if err := m.stopSiteLocked(siteID); err != nil && !strings.Contains(err.Error(), "is not running") { return 0, err }
	return m.startSiteWithEnvLocked(siteID, path, port, runtime, nil)
}

// DeploySite pulls the fast-forwardable Git revision, optionally rebuilds Go,
// then performs a stop/start cycle. Git conflicts and build errors are returned.
func (m *ProcessManager) DeploySite(siteID, path string, port int, runtime string, rebuild bool) (int, string, error) {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
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
	if stopErr := m.stopSiteLocked(siteID); stopErr != nil && !strings.Contains(stopErr.Error(), "is not running") { return 0, out, stopErr }
	pid, startErr := m.startSiteWithEnvLocked(siteID, path, port, runtime, nil)
	return pid, out, startErr
}

// GitPullSite serializes legacy in-place updates with deployments and lifecycle changes.
func (m *ProcessManager) GitPullSite(siteID string) (string, error) {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	if !validSiteID(siteID) { return "", fmt.Errorf("invalid site id") }
	root, err := safeSiteDirectory(m.DataRoot, siteID)
	if err != nil { return "", err }
	return runGitPull(root)
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


// validDeployBranch rejects option-like and path-traversal branch names before invoking Git.
func validDeployBranch(branch string) bool {
	if branch == "" || len(branch) > 128 || strings.HasPrefix(branch, "-") ||
		strings.Contains(branch, "..") || strings.Contains(branch, "//") ||
		strings.Contains(branch, "@{") || strings.HasSuffix(branch, "/") ||
		strings.HasSuffix(branch, ".") {
		return false
	}
	for _, r := range branch {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || strings.ContainsRune("._/-", r)) {
			return false
		}
	}
	return true
}

func validDeployRepository(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || strings.ContainsAny(raw, "\r\n\x00") {
		return false
	}
	if u.Scheme == "https" {
		return u.User == nil
	}
	if u.Scheme == "ssh" {
		if u.User == nil { return true }
		_, hasPassword := u.User.Password()
		return u.User.Username() == "git" && !hasPassword
	}
	return false
}

func truncateDeployOutput(value string) string {
	const max = 64 << 10
	if len(value) <= max { return value }
	return value[:max] + "\n[output truncated]"
}

// DeployGitSite builds an immutable release directory before touching the
// currently running application. A failed candidate start restores the prior
// process from its recorded working directory.
func (m *ProcessManager) DeployGitSite(req shared.SiteAction, rebuild bool) (int, string, string, string, error) {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()

	if !validSiteID(req.SiteId) { return 0, "", "", "", fmt.Errorf("invalid site id") }
	if !req.AllowProjectCode { return 0, "", "", "", fmt.Errorf("deployment blocked: explicit allow_project_code=true consent is required") }
	if !validDeployRepository(req.GitURL) { return 0, "", "", "", fmt.Errorf("Git repository must be an HTTPS or SSH URL without embedded credentials") }
	branch := req.GitBranch
	if branch == "" { branch = "main" }
	if !validDeployBranch(branch) { return 0, "", "", "", fmt.Errorf("invalid Git branch") }
	if !validHealthCheckPath(req.HealthCheckPath) { return 0, "", "", "", fmt.Errorf("invalid HTTP health check path") }
	if req.Runtime != "php" && req.Runtime != "go" && req.Runtime != "static" && req.Runtime != "nodejs" {
		return 0, "", "", "", fmt.Errorf("runtime %q cannot be launched by this Agent yet; supported runtimes are php, go, static and nodejs", req.Runtime)
	}

	siteRoot, err := safeSiteDirectory(m.DataRoot, req.SiteId)
	if err != nil { return 0, "", "", "", err }
	if err := os.MkdirAll(siteRoot, 0o750); err != nil { return 0, "", "", "", fmt.Errorf("create site directory: %w", err) }

	metadataRoot, err := ensureSafeSubdirectory(siteRoot, ".yevqo", 0o700)
	if err != nil { return 0, "", "", "", fmt.Errorf("prepare deployment metadata: %w", err) }
	stagingRoot, err := ensureSafeSubdirectory(metadataRoot, "staging", 0o700)
	if err != nil { return 0, "", "", "", fmt.Errorf("prepare deployment staging: %w", err) }
	releasesRoot, err := ensureSafeSubdirectory(metadataRoot, "releases", 0o700)
	if err != nil { return 0, "", "", "", fmt.Errorf("prepare release history: %w", err) }
	stageDir, err := os.MkdirTemp(stagingRoot, "deploy-*")
	if err != nil { return 0, "", "", "", fmt.Errorf("create deployment staging directory: %w", err) }
	defer os.RemoveAll(stageDir)

	gitEnv := append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if req.GitToken != "" {
		if len(req.GitToken) > 512 || strings.ContainsAny(req.GitToken, "\r\n\x00") {
			return 0, "", "", "", fmt.Errorf("invalid Git credential")
		}
		encoded := base64.StdEncoding.EncodeToString([]byte("x-access-token:"+req.GitToken))
		gitEnv = append(gitEnv, "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=http.extraheader",
			"GIT_CONFIG_VALUE_0=AUTHORIZATION: basic "+encoded)
	}
	// Git transports can otherwise wait indefinitely on a stalled network or
	// an unresponsive SSH endpoint. Bound checkout time independently from the
	// longer build phase so a deployment cannot hold the engine forever.
	cloneCtx, cancelClone := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancelClone()
	clone := exec.CommandContext(cloneCtx, "git", "clone", "--branch", branch, "--single-branch", "--depth", "1", req.GitURL, stageDir)
	clone.Env = gitEnv
	cloneOutput, cloneErr := clone.CombinedOutput()
	output := strings.Builder{}
	output.Write(cloneOutput)
	if cloneErr != nil {
		if cloneCtx.Err() == context.DeadlineExceeded {
			return 0, truncateDeployOutput(output.String()), "", "", fmt.Errorf("Git checkout timed out after 10 minutes")
		}
		return 0, truncateDeployOutput(output.String()), "", "", fmt.Errorf("Git checkout failed: %w", cloneErr)
	}

	commitBytes, err := exec.Command("git", "-C", stageDir, "rev-parse", "HEAD").Output()
	if err != nil { return 0, truncateDeployOutput(output.String()), "", "", fmt.Errorf("read checked-out commit: %w", err) }
	commit := strings.TrimSpace(string(commitBytes))
	if len(commit) != 40 && len(commit) != 64 { return 0, truncateDeployOutput(output.String()), "", "", fmt.Errorf("Git returned an invalid commit identifier") }
	for _, r := range commit { if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) { return 0, truncateDeployOutput(output.String()), "", "", fmt.Errorf("Git returned an invalid commit identifier") } }
	dateBytes, err := exec.Command("git", "-C", stageDir, "show", "-s", "--format=%cI", "HEAD").Output()
	if err != nil { return 0, truncateDeployOutput(output.String()), commit, "", fmt.Errorf("read commit date: %w", err) }
	commitDate := strings.TrimSpace(string(dateBytes))

	rootDir := req.RootDir
	if rootDir == "" { rootDir = "." }
	if filepath.IsAbs(rootDir) || strings.Contains(rootDir, "\\") {
		return 0, truncateDeployOutput(output.String()), commit, commitDate, fmt.Errorf("invalid root directory")
	}
	rootDir = filepath.Clean(rootDir)
	if rootDir == ".." || strings.HasPrefix(rootDir, ".."+string(filepath.Separator)) {
		return 0, truncateDeployOutput(output.String()), commit, commitDate, fmt.Errorf("root directory must stay inside the site")
	}
	buildDir, err := safeAndroidBuildRoot(stageDir, rootDir)
	if err != nil { return 0, truncateDeployOutput(output.String()), commit, commitDate, fmt.Errorf("resolve deployment root: %w", err) }

	plan, err := AnalyzeProject(buildDir)
	if err != nil { return 0, truncateDeployOutput(output.String()), commit, commitDate, err }
	expectedStack := map[string]string{"php":"php", "go":"go", "static":"static", "nodejs":"nodejs"}[req.Runtime]
	if plan.Stack != expectedStack && !(req.Runtime == "php" && plan.Stack == "unknown") {
		return 0, truncateDeployOutput(output.String()), commit, commitDate, fmt.Errorf("detected stack %q does not match configured runtime %q", plan.Stack, req.Runtime)
	}
	if req.BuildCommand != "" {
		return 0, truncateDeployOutput(output.String()), commit, commitDate, fmt.Errorf("custom shell build commands are disabled; use a supported detected stack")
	}
	if req.Runtime == "php" && plan.Stack == "unknown" {
		// Plain PHP sites without Composer have no dependency/build phase.
	} else {
		if m.BuildRunner == nil { m.BuildRunner = NewAndroidBuildRunner(m.DataRoot) }
		build := m.BuildRunner.Run(context.Background(), AndroidBuildRequest{
			SiteID:req.SiteId, SiteRoot:stageDir, RootDir:rootDir, Stack:plan.Stack,
			PackageTool:plan.PackageTool, AllowProjectCode:true, Timeout:20*time.Minute, MaxLogBytes:10<<20,
		})
		output.WriteString("\n")
		output.WriteString(build.Output)
		if !build.Success {
			return 0, truncateDeployOutput(output.String()), commit, commitDate, fmt.Errorf("build failed: %s", build.Error)
		}
	}

	releaseDir := filepath.Join(releasesRoot, commit)
	if _, statErr := os.Stat(releaseDir); statErr == nil {
		return 0, truncateDeployOutput(output.String()), commit, commitDate, fmt.Errorf("release %s already exists; refusing to overwrite it", commit)
	} else if !os.IsNotExist(statErr) {
		return 0, truncateDeployOutput(output.String()), commit, commitDate, statErr
	}
	if err := os.Rename(stageDir, releaseDir); err != nil {
		return 0, truncateDeployOutput(output.String()), commit, commitDate, fmt.Errorf("finalize immutable release: %w", err)
	}

	var previous ProcessRecord
	hadPrevious := false
	if m.registry != nil {
		previous, hadPrevious = m.registry.Get(req.SiteId)
	}
	if hadPrevious && verifyProcessRecord(previous) {
		if err := m.stopSiteLocked(req.SiteId); err != nil {
			return 0, truncateDeployOutput(output.String()), commit, commitDate, fmt.Errorf("stop previous release: %w", err)
		}
	} else if hadPrevious {
		_ = m.registry.Delete(req.SiteId)
		m.mu.Lock()
		delete(m.procs, req.SiteId)
		m.mu.Unlock()
		hadPrevious = false
	}

	runPath := filepath.Join("sites", req.SiteId, ".yevqo", "releases", commit)
	if rootDir != "." { runPath = filepath.Join(runPath, rootDir) }
	pid, startErr := m.startSiteWithEnvLocked(req.SiteId, runPath, int(req.Port), req.Runtime, nil)
	if startErr == nil && req.HealthCheckPath != "" {
		if healthErr := waitForHTTP(int(req.Port), req.HealthCheckPath, 15*time.Second); healthErr != nil {
			_ = m.stopSiteLocked(req.SiteId)
			startErr = healthErr
		}
	}
	if startErr != nil {
		if hadPrevious {
			oldRunPath := previous.RunPath
			if oldRunPath == "" {
				rel, relErr := filepath.Rel(siteRoot, previous.WorkingDir)
				if relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					oldRunPath = filepath.Join("sites", req.SiteId, rel)
				}
			}
			if oldRunPath == "" {
				return 0, truncateDeployOutput(output.String()), commit, commitDate, fmt.Errorf("new release failed to start (%v); rollback also failed (previous release path is unavailable)", startErr)
			}
			if _, rollbackErr := m.startSiteWithEnvLocked(req.SiteId, oldRunPath, previous.Port, previous.Runtime, nil); rollbackErr != nil {
				return 0, truncateDeployOutput(output.String()), commit, commitDate, fmt.Errorf("new release failed to start (%v); rollback also failed (%v)", startErr, rollbackErr)
			}
			return 0, truncateDeployOutput(output.String()), commit, commitDate, fmt.Errorf("new release failed health check; previous release was restored: %w", startErr)
		}
		return 0, truncateDeployOutput(output.String()), commit, commitDate, fmt.Errorf("new release failed health check and no previous release was active: %w", startErr)
	}
	output.WriteString(fmt.Sprintf("\nActivated release %s (pid %d)", commit, pid))
	return pid, truncateDeployOutput(output.String()), commit, commitDate, nil
}



func validReleaseCommit(commit string) bool {
	if len(commit) != 40 && len(commit) != 64 { return false }
	for _, r := range commit { if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) { return false } }
	return true
}

// ListSiteReleases reads immutable release directories and marks the release
// currently referenced by the durable process registry.
func (m *ProcessManager) ListSiteReleases(siteID, runtime string) ([]shared.SiteRelease, error) {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	if !validSiteID(siteID) { return nil, fmt.Errorf("invalid site id") }
	root, err := safeSiteDirectory(m.DataRoot, siteID)
	if err != nil { return nil, err }
	releasesRoot, err := safeExistingSubdirectory(root, filepath.Join(".yevqo", "releases"))
	if os.IsNotExist(err) { return []shared.SiteRelease{}, nil }
	if err != nil { return nil, fmt.Errorf("resolve release history: %w", err) }
	entries, err := os.ReadDir(releasesRoot)
	if err != nil { return nil, fmt.Errorf("read release history: %w", err) }
	activeCommit := ""
	if m.registry != nil {
		if record, ok := m.registry.Get(siteID); ok {
			prefix := filepath.Join("sites", siteID, ".yevqo", "releases") + string(filepath.Separator)
			rel := filepath.Clean(record.RunPath)
			if strings.HasPrefix(rel, prefix) {
				rest := strings.TrimPrefix(rel, prefix)
				activeCommit = strings.SplitN(rest, string(filepath.Separator), 2)[0]
			}
		}
	}
	out := make([]shared.SiteRelease, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !validReleaseCommit(entry.Name()) { continue }
		releaseDir := filepath.Join(releasesRoot, entry.Name())
		resolved, resolveErr := filepath.EvalSymlinks(releaseDir)
		if resolveErr != nil { continue }
		rel, relErr := filepath.Rel(releasesRoot, resolved)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) { continue }
		date := ""
		if dateBytes, gitErr := exec.Command("git", "-C", resolved, "show", "-s", "--format=%cI", "HEAD").Output(); gitErr == nil {
			date = strings.TrimSpace(string(dateBytes))
		}
		out = append(out, shared.SiteRelease{Commit:entry.Name(), Date:date, Active:entry.Name()==activeCommit, Runtime:runtime})
	}
	sort.Slice(out, func(i,j int) bool {
		if out[i].Date == out[j].Date { return out[i].Commit > out[j].Commit }
		return out[i].Date > out[j].Date
	})
	if len(out) > 50 { out = out[:50] }
	return out, nil
}

// RollbackSiteRelease switches to an existing immutable release without
// rebuilding it. If activation fails, the previously running process is restored.
func (m *ProcessManager) RollbackSiteRelease(siteID, commit, rootDir, runtime string, port int) (int, error) {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	if !validSiteID(siteID) { return 0, fmt.Errorf("invalid site id") }
	if !validReleaseCommit(commit) { return 0, fmt.Errorf("invalid release commit") }
	if runtime != "php" && runtime != "go" && runtime != "static" && runtime != "nodejs" { return 0, fmt.Errorf("unsupported runtime: %s", runtime) }
	if port < 1024 || port > 65535 { return 0, fmt.Errorf("invalid site port") }
	if rootDir == "" { rootDir = "." }
	if filepath.IsAbs(rootDir) || strings.Contains(rootDir, "\\") { return 0, fmt.Errorf("invalid release root directory") }
	rootDir = filepath.Clean(rootDir)
	if rootDir == ".." || strings.HasPrefix(rootDir, ".."+string(filepath.Separator)) { return 0, fmt.Errorf("release root directory escapes site") }
	siteRoot, err := safeSiteDirectory(m.DataRoot, siteID)
	if err != nil { return 0, err }
	releasesRoot, err := safeExistingSubdirectory(siteRoot, filepath.Join(".yevqo", "releases"))
	if err != nil { return 0, fmt.Errorf("release history not found: %w", err) }
	resolvedReleasesRoot, err := filepath.EvalSymlinks(releasesRoot)
	if err != nil { return 0, fmt.Errorf("resolve release history: %w", err) }
	releaseDir := filepath.Join(releasesRoot, commit)
	releaseInfoOnDisk, err := os.Lstat(releaseDir)
	if err != nil { return 0, fmt.Errorf("release not found: %s", commit) }
	if !releaseInfoOnDisk.IsDir() || releaseInfoOnDisk.Mode()&os.ModeSymlink != 0 {
		return 0, fmt.Errorf("release must be a real directory")
	}
	resolvedRelease, err := filepath.EvalSymlinks(releaseDir)
	if err != nil { return 0, fmt.Errorf("resolve release %s: %w", commit, err) }
	releaseRel, err := filepath.Rel(resolvedReleasesRoot, resolvedRelease)
	if err != nil || releaseRel == "." || releaseRel == ".." || strings.HasPrefix(releaseRel, ".."+string(filepath.Separator)) {
		return 0, fmt.Errorf("release path resolves outside release history")
	}
	releaseInfo, err := os.Stat(resolvedRelease)
	if err != nil || !releaseInfo.IsDir() { return 0, fmt.Errorf("release is not a directory") }
	resolvedRoot, err := filepath.EvalSymlinks(filepath.Join(resolvedRelease, rootDir))
	if err != nil { return 0, fmt.Errorf("release root directory not found: %w", err) }
	rel, err := filepath.Rel(resolvedRelease, resolvedRoot)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) { return 0, fmt.Errorf("release root resolves outside release directory") }
	if m.registry == nil || m.registryErr != nil { return 0, fmt.Errorf("process registry unavailable: %v", m.registryErr) }
	previous, hadPrevious := m.registry.Get(siteID)
	if hadPrevious && verifyProcessRecord(previous) {
		if err := m.stopSiteLocked(siteID); err != nil { return 0, fmt.Errorf("stop current release: %w", err) }
	} else if hadPrevious {
		_ = m.registry.Delete(siteID)
		m.mu.Lock(); delete(m.procs, siteID); m.mu.Unlock()
		hadPrevious = false
	}
	runPath := filepath.Join("sites", siteID, ".yevqo", "releases", commit)
	if rootDir != "." { runPath = filepath.Join(runPath, rootDir) }
	pid, startErr := m.startSiteWithEnvLocked(siteID, runPath, port, runtime, nil)
	if startErr == nil { return pid, nil }
	if hadPrevious {
		oldPath := previous.RunPath
		if oldPath == "" {
			if rel, e := filepath.Rel(siteRoot, previous.WorkingDir); e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				oldPath = filepath.Join("sites", siteID, rel)
			}
		}
		if oldPath == "" { return 0, fmt.Errorf("rollback activation failed (%v); previous process path unavailable", startErr) }
		if _, restoreErr := m.startSiteWithEnvLocked(siteID, oldPath, previous.Port, previous.Runtime, nil); restoreErr != nil {
			return 0, fmt.Errorf("rollback activation failed (%v); restoring previous release also failed (%v)", startErr, restoreErr)
		}
	}
	return 0, fmt.Errorf("could not activate release %s: %w", commit, startErr)
}

// BuildSite runs the Android-native local provider for a site. Project code is
// executed only when the caller explicitly sets AllowProjectCode.
func (m *ProcessManager) BuildSite(req AndroidBuildRequest) AndroidBuildResult {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	if !validSiteID(req.SiteID) {
		return AndroidBuildResult{Provider: "android-local", Stack: req.Stack, Error: "invalid site id"}
	}
	siteRoot, err := safeSiteDirectory(m.DataRoot, req.SiteID)
	if err != nil {
		return AndroidBuildResult{Provider: "android-local", Stack: req.Stack, Error: err.Error()}
	}
	req.SiteRoot = siteRoot
	if m.BuildRunner == nil {
		m.BuildRunner = NewAndroidBuildRunner(m.DataRoot)
	}
	return m.BuildRunner.Run(context.Background(), req)
}
