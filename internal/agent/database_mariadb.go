package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"syscall"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"veltrix/internal/shared"
)

var mariaIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}$`)

func mariaCLI() (string, error) {
	for _, name := range []string{"mariadb", "mysql"} {
		if path, err := exec.LookPath(name); err == nil { return path, nil }
	}
	return "", fmt.Errorf("MariaDB client is not installed")
}
func mariaDumpCLI() (string, error) {
	for _, name := range []string{"mariadb-dump", "mysqldump"} {
		if path, err := exec.LookPath(name); err == nil { return path, nil }
	}
	return "", fmt.Errorf("mariadb-dump is not installed")
}
func mariaSocket(dataDir string) string { return filepath.Join(dataDir, "yevqo-mariadb.sock") }
func mariaDataDirectory(prefix string, serverID int64, configured string) (string, error) {
	if prefix == "" { return "", fmt.Errorf("Termux PREFIX is not set") }; if configured == "" { return "", fmt.Errorf("configured MariaDB data directory is required") }
	return resolveTermuxPath(prefix, configured)
}
func mariaAdminArgs(dataDir string) []string {
	name := "root"
	return []string{"--protocol=SOCKET", "--socket=" + mariaSocket(dataDir), "-u", name}
}
func runMariaSQL(ctx context.Context, dataDir, database, sqlText string) error {
	cli, err := mariaCLI(); if err != nil { return err }
	args := append(mariaAdminArgs(dataDir), "--batch", "--skip-column-names")
	if database != "" { args = append(args, database) }
	cmd := exec.CommandContext(ctx, cli, args...)
	cmd.Stdin = strings.NewReader(sqlText)
	var stderr bytes.Buffer; cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil { return fmt.Errorf("MariaDB command timed out") }
		msg := trimOutput(stderr.String()); if msg == "" { return fmt.Errorf("MariaDB operation failed") }
		return fmt.Errorf("MariaDB operation failed: %s", msg)
	}
	return nil
}
func mariaSQLLiteral(v string) string { v = strings.ReplaceAll(v, "\\", "\\\\"); return "'" + strings.ReplaceAll(v, "'", "''") + "'" }
func mariaQuoteIdentifier(v string) string { return "`" + strings.ReplaceAll(v, "`", "``") + "`" }

func installMariaDBServer(req shared.InstallDatabaseServer) (*shared.DatabaseServerInstallResult, error) {
	if req.Port < 1024 || req.Port > 65535 || req.MaxConnections < 1 || req.MaxConnections > 1000 { return nil, fmt.Errorf("invalid MariaDB port or connection limit") }; if req.SSLMode != "" && req.SSLMode != "disable" && req.SSLMode != "prefer" && req.SSLMode != "require" { return nil, fmt.Errorf("unsupported MariaDB SSL mode") }
	prefix := os.Getenv("PREFIX"); if prefix == "" { return nil, fmt.Errorf("Termux PREFIX is not set; run the Agent inside Termux") }
	dataDir, err := resolveTermuxPath(prefix, req.DataDirectory); if err != nil { return nil, err }
	if l, err := netListenLocal(req.Port); err != nil { return nil, err } else { _ = l }
	var fs syscallStatfs
	if getTermuxFreeSpace(prefix, &fs) && fs.available < 200*1024*1024 { return nil, fmt.Errorf("at least 200 MB of free storage is required") }
	if info, err := os.Lstat(dataDir); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() { return nil, fmt.Errorf("MariaDB data path must be a real directory") }
		entries, e := os.ReadDir(dataDir); if e != nil { return nil, e }
		if len(entries) > 0 {
			if _, e := os.Stat(filepath.Join(dataDir, "ibdata1")); e == nil { return nil, fmt.Errorf("an existing MariaDB data directory was detected; it was not modified") }
			return nil, fmt.Errorf("MariaDB data directory is not empty; no files were changed")
		}
	} else if !os.IsNotExist(err) { return nil, err }
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute); defer cancel()
	installDB, err := exec.LookPath("mariadb-install-db")
	if err != nil { installDB, err = exec.LookPath("mysql_install_db") }
	if err != nil {
		pkg, e := exec.LookPath("pkg"); if e != nil { return nil, fmt.Errorf("mariadb-install-db is missing and Termux pkg is unavailable") }
		out, e := exec.CommandContext(ctx, pkg, "install", "-y", "mariadb").CombinedOutput()
		if e != nil { return nil, fmt.Errorf("Termux MariaDB installation failed: %s", trimOutput(string(out))) }
		installDB, err = exec.LookPath("mariadb-install-db"); if err != nil { installDB, err = exec.LookPath("mysql_install_db") }
		if err != nil { return nil, fmt.Errorf("MariaDB package installed but mariadb-install-db is unavailable") }
	}
	if err := os.MkdirAll(filepath.Dir(dataDir), 0o700); err != nil { return nil, err }
	args := []string{"--datadir=" + dataDir, "--auth-root-authentication-method=normal", "--skip-test-db"}
	
	out, err := exec.CommandContext(ctx, installDB, args...).CombinedOutput()
	if err != nil { return nil, fmt.Errorf("MariaDB system-table initialization failed: %s", trimOutput(string(out))) }
	if err := startMariaDB(dataDir, req.Port, req.MaxConnections, req.SSLMode); err != nil { return nil, err }
	cli, err := mariaCLI(); if err != nil { return nil, err }
	versionOut, _ := exec.CommandContext(ctx, cli, "--version").CombinedOutput()
	version := strings.TrimSpace(string(versionOut)); if version == "" { version = "MariaDB (version unavailable)" }
	return &shared.DatabaseServerInstallResult{Version: version, DataDirectory: dataDir, Port: req.Port}, nil
}

// Small wrappers keep platform-specific filesystem probing isolated from the database lifecycle.
type localListener interface{ Close() error }
func netListenLocal(port uint32) (localListener, error) {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil { return nil, fmt.Errorf("port %d is already in use", port) }
	return l, nil
}
type syscallStatfs struct{ available uint64 }
func getTermuxFreeSpace(path string, out *syscallStatfs) bool {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil { return false }
	out.available = st.Bavail * uint64(st.Bsize); return true
}

func ensureMariaDBSSL(dataDir string) (string, string, error) {
	cert := filepath.Join(dataDir, "server.crt")
	key := filepath.Join(dataDir, "server.key")
	certInfo, certErr := os.Lstat(cert); keyInfo, keyErr := os.Lstat(key)
	if certErr == nil || keyErr == nil {
		if certErr != nil || keyErr != nil || certInfo.Mode()&os.ModeSymlink != 0 || keyInfo.Mode()&os.ModeSymlink != 0 || !certInfo.Mode().IsRegular() || !keyInfo.Mode().IsRegular() { return "", "", fmt.Errorf("existing MariaDB SSL certificate paths are unsafe") }
		if err := os.Chmod(key, 0o600); err != nil { return "", "", err }
		return cert, key, nil
	}
	if !os.IsNotExist(certErr) || !os.IsNotExist(keyErr) { return "", "", fmt.Errorf("could not inspect MariaDB SSL certificate paths") }
	openssl, err := exec.LookPath("openssl"); if err != nil { return "", "", fmt.Errorf("openssl is required for MariaDB SSL; install the Termux openssl package") }
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second); defer cancel()
	out, err := exec.CommandContext(ctx, openssl, "req", "-newkey", "rsa:2048", "-nodes", "-keyout", key, "-x509", "-days", "3650", "-out", cert, "-subj", "/CN=localhost", "-addext", "subjectAltName=DNS:localhost,IP:127.0.0.1").CombinedOutput()
	if err != nil { _ = os.Remove(cert); _ = os.Remove(key); return "", "", fmt.Errorf("generate MariaDB SSL certificate: %s", trimOutput(string(out))) }
	if err := os.Chmod(key, 0o600); err != nil { return "", "", err }; if err := os.Chmod(cert, 0o644); err != nil { return "", "", err }
	return cert, key, nil
}

func startMariaDB(dataDir string, port uint32, maxConnections uint32, sslMode string) error {
	safe, err := exec.LookPath("mariadbd-safe"); if err != nil { safe, err = exec.LookPath("mysqld_safe") }
	if err != nil { return fmt.Errorf("mariadbd-safe is not installed") }
	if err := os.MkdirAll(filepath.Join(dataDir, "run"), 0o700); err != nil { return err }
	logPath := filepath.Join(dataDir, "mariadb.log")
	args := []string{"--datadir=" + dataDir, "--socket=" + mariaSocket(dataDir), "--pid-file=" + filepath.Join(dataDir, "run", "mariadb.pid"), "--port=" + strconv.Itoa(int(port)), "--bind-address=127.0.0.1", "--max-connections=" + strconv.Itoa(int(maxConnections)), "--log-error=" + logPath, "--skip-name-resolve"}
	if sslMode == "" || sslMode == "require" || sslMode == "prefer" { cert,key,sslErr := ensureMariaDBSSL(dataDir); if sslErr != nil { return sslErr }; args = append(args, "--ssl-ca="+cert, "--ssl-cert="+cert, "--ssl-key="+key) } else if sslMode != "disable" { return fmt.Errorf("unsupported MariaDB SSL mode") }
	if _, err := os.Stat(mariaSocket(dataDir)); err == nil { if waitMariaDB(dataDir, 3*time.Second) == nil { return nil } }
	cmd := exec.Command(safe, args...); cmd.Stdout = io.Discard; cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil { return fmt.Errorf("could not start MariaDB: %w", err) }
	return waitMariaDB(dataDir, 45*time.Second)
}
func waitMariaDB(dataDir string, timeout time.Duration) error {
	cli, err := exec.LookPath("mariadb-admin"); if err != nil { cli, err = exec.LookPath("mysqladmin") }
	if err != nil { return fmt.Errorf("mariadb-admin is not installed") }
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		args := append(mariaAdminArgs(dataDir), "ping")
		out, e := exec.CommandContext(ctx, cli, args...).CombinedOutput(); cancel()
		if e == nil && strings.Contains(strings.ToLower(string(out)), "alive") { return nil }
		time.Sleep(500*time.Millisecond)
	}
	return fmt.Errorf("MariaDB did not become ready; inspect %s", filepath.Join(dataDir, "mariadb.log"))
}
func manageMariaDBServer(req shared.DatabaseServerAction, action string) error {
	if req.ServerID < 1 || req.Port < 1024 || req.Port > 65535 || (action != "start" && action != "stop") { return fmt.Errorf("invalid MariaDB lifecycle request") }
	prefix := os.Getenv("PREFIX"); if prefix == "" { return fmt.Errorf("Termux PREFIX is not set") }
	dataDir, err := resolveTermuxPath(prefix, req.DataDirectory); if err != nil { return err }
	info, err := os.Lstat(dataDir); if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() { return fmt.Errorf("MariaDB data directory is unavailable or unsafe") }
	if action == "start" { max := req.MaxConnections; if max < 1 { max = 100 }; return startMariaDB(dataDir, req.Port, max, req.SSLMode) }
	admin, err := exec.LookPath("mariadb-admin"); if err != nil { admin, err = exec.LookPath("mysqladmin") }; if err != nil { return fmt.Errorf("mariadb-admin is not installed") }
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second); defer cancel()
	out, err := exec.CommandContext(ctx, admin, append(mariaAdminArgs(dataDir), "shutdown")...).CombinedOutput()
	if err != nil { if ctx.Err() != nil { return fmt.Errorf("MariaDB shutdown timed out") }; if strings.Contains(strings.ToLower(string(out)), "connect") { return nil }; return fmt.Errorf("MariaDB shutdown failed: %s", trimOutput(string(out))) }
	return nil
}

func createMariaDBDatabase(req shared.CreateDatabase) error {
	if req.DatabaseID < 1 || req.ServerID < 1 || req.Port < 1024 || req.Port > 65535 || !mariaIdentifier.MatchString(req.Database) || !mariaIdentifier.MatchString(req.Username) || len(req.Password) < 12 || len(req.Password) > 256 || strings.IndexByte(req.Password, 0) >= 0 { return fmt.Errorf("invalid MariaDB database request") }
	prefix := os.Getenv("PREFIX"); if prefix == "" { return fmt.Errorf("Termux PREFIX is not set") }
	dataDir, err := mariaDataDirectory(prefix, req.ServerID, req.DataDirectory); if err != nil { return err }
	return provisionMariaDBDatabase(dataDir, req)
}

func provisionMariaDBDatabase(dataDir string, req shared.CreateDatabase) error {
	dbID := mariaQuoteIdentifier(req.Database)
	userID := mariaQuoteIdentifier(req.Username) + "@'127.0.0.1'"
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second); defer cancel()
	if err := runMariaSQL(ctx, dataDir, "", "CREATE DATABASE "+dbID+" CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"); err != nil { return err }
	createUserSQL := "CREATE USER "+userID+" IDENTIFIED BY "+mariaSQLLiteral(req.Password); if req.SSLMode == "require" { createUserSQL += " REQUIRE SSL" }; if err := runMariaSQL(ctx, dataDir, "", createUserSQL+";"); err != nil {
		_ = runMariaSQL(ctx, dataDir, "", "DROP DATABASE IF EXISTS "+dbID+";")
		return err
	}
	if err := runMariaSQL(ctx, dataDir, "", "GRANT ALL PRIVILEGES ON "+dbID+".* TO "+userID+";"); err != nil {
		_ = runMariaSQL(ctx, dataDir, "", "DROP DATABASE IF EXISTS "+dbID+"; DROP USER IF EXISTS "+userID+";")
		return err
	}
	return nil
}

func testMariaDBConnection(req shared.TestDatabaseConnection) error {
	if req.DatabaseID < 1 || !mariaIdentifier.MatchString(req.Database) || !mariaIdentifier.MatchString(req.Username) || req.Port < 1024 || req.Port > 65535 || len(req.Password) < 12 || len(req.Password) > 256 { return fmt.Errorf("invalid MariaDB connection request") }
	cli, err := mariaCLI(); if err != nil { return err }
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second); defer cancel()
	args := []string{"--protocol=TCP", "-h", "127.0.0.1", "-P", fmt.Sprint(req.Port), "-u", req.Username}
	if req.SSLMode == "require" { prefix := os.Getenv("PREFIX"); dataDir, pathErr := mariaDataDirectory(prefix, 0, req.DataDirectory); if pathErr != nil { return pathErr }; cert, _, certErr := ensureMariaDBSSL(dataDir); if certErr != nil { return certErr }; args = append(args, "--ssl", "--ssl-ca="+cert, "--ssl-verify-server-cert") } else if req.SSLMode == "disable" { args = append(args, "--skip-ssl") } else if req.SSLMode != "" && req.SSLMode != "prefer" { return fmt.Errorf("invalid MariaDB SSL mode") }
	args = append(args, req.Database, "-Nse", "SELECT 1")
	cmd := exec.CommandContext(ctx, cli, args...)
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+req.Password)
	out, err := cmd.Output(); if err != nil { if ctx.Err() != nil { return fmt.Errorf("MariaDB connection test timed out") }; return fmt.Errorf("MariaDB connection test failed") }
	if strings.TrimSpace(string(out)) != "1" { return fmt.Errorf("MariaDB returned an unexpected connection test result") }
	return nil
}

func manageMariaDBRole(req shared.ManageDatabaseRole, action string) error {
	if req.ServerID < 1 || req.Port < 1024 || req.Port > 65535 || !mariaIdentifier.MatchString(req.Name) || req.Name == "root" { return fmt.Errorf("invalid MariaDB user request") }
	if action != "create" && action != "drop" && action != "password" { return fmt.Errorf("unsupported MariaDB user action") }
	if (action == "create" || action == "password") && (len(req.Password) < 12 || len(req.Password) > 256) { return fmt.Errorf("user password must be 12–256 characters") }
	prefix := os.Getenv("PREFIX"); dataDir, err := mariaDataDirectory(prefix, req.ServerID, req.DataDirectory); if err != nil { return err }
	userHost := mariaQuoteIdentifier(req.Name) + "@'127.0.0.1'"
	query := ""
	switch action { case "create": query = "CREATE USER " + userHost + " IDENTIFIED BY " + mariaSQLLiteral(req.Password); if req.SSLMode == "require" { query += " REQUIRE SSL" }
	case "password": query = "ALTER USER " + userHost + " IDENTIFIED BY " + mariaSQLLiteral(req.Password)
	case "drop": query = "DROP USER " + userHost }
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second); defer cancel()
	return runMariaSQL(ctx, dataDir, "", query+";")
}

func listMariaDBRoles(req shared.ListDatabaseRoles) ([]shared.DatabaseRole, error) {
	if req.ServerID < 1 || req.Port < 1024 || req.Port > 65535 { return nil, fmt.Errorf("invalid MariaDB user inventory request") }
	prefix := os.Getenv("PREFIX"); dataDir, err := mariaDataDirectory(prefix, req.ServerID, req.DataDirectory); if err != nil { return nil, err }
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second); defer cancel()
	cli, err := mariaCLI(); if err != nil { return nil, err }
	query := "SELECT JSON_ARRAYAGG(JSON_OBJECT('name',User,'can_login',true,'is_superuser',false,'is_replication',false,'bypass_rls',false,'inherit',true,'connection_limit',0)) FROM mysql.user WHERE User NOT IN ('root','mariadb.sys','mysql');"
	cmd := exec.CommandContext(ctx, cli, append(mariaAdminArgs(dataDir), "-Nse", query)...)
	out, err := cmd.Output(); if err != nil { return nil, fmt.Errorf("could not read MariaDB users") }
	var roles []shared.DatabaseRole
	raw := strings.TrimSpace(string(out)); if raw == "" || raw == "NULL" { return []shared.DatabaseRole{}, nil }
	if err := json.Unmarshal([]byte(raw), &roles); err != nil { return nil, fmt.Errorf("invalid MariaDB user inventory") }
	return roles, nil
}

func manageMariaDBPrivilege(req shared.ManageDatabasePrivilege) error {
	if req.ServerID < 1 || req.Port < 1024 || req.Port > 65535 || !mariaIdentifier.MatchString(req.Database) || !mariaIdentifier.MatchString(req.Role) || req.Role == "root" { return fmt.Errorf("invalid MariaDB privilege request") }
	if req.Action != "grant" && req.Action != "revoke" { return fmt.Errorf("action must be grant or revoke") }
	allowed := map[string]bool{"SELECT":true,"INSERT":true,"UPDATE":true,"DELETE":true,"CREATE":true,"DROP":true,"INDEX":true,"ALTER":true,"REFERENCES":true,"EXECUTE":true,"TRIGGER":true}
	if !allowed[req.Privilege] { return fmt.Errorf("unsupported MariaDB privilege") }
	if req.Scope != "" && req.Scope != "database" { return fmt.Errorf("MariaDB privilege scope must be database") }
	prefix := os.Getenv("PREFIX"); dataDir, err := mariaDataDirectory(prefix, req.ServerID, req.DataDirectory); if err != nil { return err }
	statement := "GRANT " + req.Privilege + " ON " + mariaQuoteIdentifier(req.Database) + ".* TO " + mariaQuoteIdentifier(req.Role) + "@'127.0.0.1'"
	if req.Action == "revoke" { statement = "REVOKE " + req.Privilege + " ON " + mariaQuoteIdentifier(req.Database) + ".* FROM " + mariaQuoteIdentifier(req.Role) + "@'127.0.0.1'" }
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second); defer cancel()
	return runMariaSQL(ctx, dataDir, "", statement+";")
}

func collectMariaDBStats(req shared.GetDatabaseStats) (shared.DatabaseStats, error) {
	if req.ServerID < 1 || req.Port < 1024 || req.Port > 65535 || !mariaIdentifier.MatchString(req.Database) { return shared.DatabaseStats{}, fmt.Errorf("invalid MariaDB stats request") }
	prefix := os.Getenv("PREFIX"); dataDir, err := mariaDataDirectory(prefix, req.ServerID, req.DataDirectory); if err != nil { return shared.DatabaseStats{}, err }
	cli, err := mariaCLI(); if err != nil { return shared.DatabaseStats{}, err }
	query := "SELECT JSON_OBJECT('size_bytes',COALESCE(SUM(data_length+index_length),0),'active_connections',(SELECT COUNT(*) FROM information_schema.PROCESSLIST WHERE DB=DATABASE()),'max_connections',@@max_connections,'transactions_committed',0,'transactions_rolled_back',0,'deadlocks',0) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE();"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second); defer cancel()
	cmd := exec.CommandContext(ctx, cli, append(mariaAdminArgs(dataDir), req.Database, "-Nse", query)...)
	out, err := cmd.Output(); if err != nil { return shared.DatabaseStats{}, fmt.Errorf("could not query MariaDB database stats") }
	var stats shared.DatabaseStats
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(out))), &stats); err != nil { return stats, fmt.Errorf("invalid MariaDB stats response") }
	stats.CapturedAt = time.Now().UTC(); return stats, nil
}

func createMariaDBBackup(req shared.CreateDatabaseBackup) (*shared.DatabaseBackupResult, error) {
	if req.DatabaseID < 1 || req.ServerID < 1 || req.Port < 1024 || req.Port > 65535 || !mariaIdentifier.MatchString(req.Database) { return nil, fmt.Errorf("invalid MariaDB backup request") }
	prefix := os.Getenv("PREFIX"); if prefix == "" { return nil, fmt.Errorf("Termux PREFIX is not set") }
	dataDir, err := mariaDataDirectory(prefix, req.ServerID, req.DataDirectory); if err != nil { return nil, err }
	dump, err := mariaDumpCLI(); if err != nil { return nil, err }
	backupDir := filepath.Join(prefix, "var", "lib", "veltrix-agent", "backups", "databases", fmt.Sprint(req.ServerID))
	if err := os.MkdirAll(backupDir, 0o700); err != nil { return nil, err }
	root, err := filepath.EvalSymlinks(prefix); if err != nil { return nil, err }
	realDir, err := filepath.EvalSymlinks(backupDir); if err != nil { return nil, err }
	rel, err := filepath.Rel(root, realDir); if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) { return nil, fmt.Errorf("backup path escapes Termux private storage") }
	name := fmt.Sprintf("%s-%d-%s.sql", req.Database, req.DatabaseID, time.Now().UTC().Format("20060102T150405.000000Z"))
	path := filepath.Join(backupDir, name)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600); if err != nil { return nil, err }
	ok := false; defer func(){ if !ok { _ = file.Close(); _ = os.Remove(path) } }()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute); defer cancel()
	cmd := exec.CommandContext(ctx, dump, append(mariaAdminArgs(dataDir), "--single-transaction", "--routines", "--triggers", "--events", "--hex-blob", req.Database)...)
	cmd.Stdout = file; var stderr bytes.Buffer; cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil { return nil, fmt.Errorf("MariaDB backup failed: %s", trimOutput(stderr.String())) }
	if err := file.Sync(); err != nil { return nil, err }; if err := file.Close(); err != nil { return nil, err }
	info, err := os.Stat(path); if err != nil { return nil, err }; if info.Size() < 64 { return nil, fmt.Errorf("MariaDB backup is unexpectedly small") }
	check, err := os.Open(path); if err != nil { return nil, err }; scanner := bufio.NewScanner(io.LimitReader(check, 4096)); scanner.Buffer(make([]byte, 1024), 4096)
	foundHeader := false; for scanner.Scan() { if strings.Contains(scanner.Text(), "MariaDB dump") || strings.Contains(scanner.Text(), "MySQL dump") { foundHeader = true; break } }; _ = check.Close()
	if !foundHeader { return nil, fmt.Errorf("MariaDB backup verification failed") }
	ok = true
	return &shared.DatabaseBackupResult{FileName:name, SizeBytes:info.Size(), Format:"sql", Verified:true}, nil
}

func restoreMariaDBBackup(req shared.RestoreDatabaseBackup) error {
	if req.BackupServerID < 1 || req.Database.ServerID < 1 || req.Database.Port < 1024 || req.Database.Port > 65535 || !mariaIdentifier.MatchString(req.Database.Database) || !mariaIdentifier.MatchString(req.Database.Username) || len(req.Database.Password) < 12 || len(req.Database.Password) > 256 || !mariaBackupFilePattern.MatchString(req.BackupFileName) || filepath.Base(req.BackupFileName) != req.BackupFileName { return fmt.Errorf("invalid MariaDB restore request") }
	prefix := os.Getenv("PREFIX"); if prefix == "" { return fmt.Errorf("Termux PREFIX is not set") }
	backupDir := filepath.Join(prefix, "var", "lib", "veltrix-agent", "backups", "databases", fmt.Sprint(req.BackupServerID))
	full := filepath.Join(backupDir, req.BackupFileName)
	info, err := os.Lstat(full); if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() { return fmt.Errorf("MariaDB backup file not found or unsafe") }
	root, err := filepath.EvalSymlinks(prefix); if err != nil { return err }; realDir, err := filepath.EvalSymlinks(backupDir); if err != nil { return err }
	rel, err := filepath.Rel(root, realDir); if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) { return fmt.Errorf("backup directory escapes private storage") }
	dataDir, err := mariaDataDirectory(prefix, req.Database.ServerID, req.Database.DataDirectory); if err != nil { return err }
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute); defer cancel()
	if err := provisionMariaDBDatabase(dataDir, req.Database); err != nil { return err }
	cli, err := mariaCLI(); if err != nil { _ = runMariaSQL(context.Background(), dataDir, "", "DROP DATABASE "+mariaQuoteIdentifier(req.Database.Database)+"; DROP USER "+mariaQuoteIdentifier(req.Database.Username)+"@'127.0.0.1';"); return err }
	f, err := os.Open(full); if err != nil { cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), 30*time.Second); _ = runMariaSQL(cleanupCtx, dataDir, "", "DROP DATABASE IF EXISTS "+mariaQuoteIdentifier(req.Database.Database)+"; DROP USER IF EXISTS "+mariaQuoteIdentifier(req.Database.Username)+"@'127.0.0.1';"); cancelCleanup(); return err }
	reader, writer := io.Pipe()
	go func() { rewriteErr := rewriteMariaDBDump(f, writer, req.Database.Database); _ = writer.CloseWithError(rewriteErr); _ = f.Close() }()
	cmd := exec.CommandContext(ctx, cli, append(mariaAdminArgs(dataDir), req.Database.Database)...); cmd.Stdin = reader
	var stderr bytes.Buffer; cmd.Stderr = &stderr
	err = cmd.Run(); _ = reader.Close()
	if err != nil {
		cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), 30*time.Second)
		_ = runMariaSQL(cleanupCtx, dataDir, "", "DROP DATABASE IF EXISTS "+mariaQuoteIdentifier(req.Database.Database)+"; DROP USER IF EXISTS "+mariaQuoteIdentifier(req.Database.Username)+"@'127.0.0.1';")
		cancelCleanup()
		return fmt.Errorf("MariaDB restore failed; newly created target database was cleaned up: %s", trimOutput(stderr.String()))
	}
	return nil
}

var mariaBackupFilePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}-[0-9]+-[0-9]{8}T[0-9]{6}\.[0-9]{6}Z\.sql$`)

func rewriteMariaDBDump(source io.Reader, destination io.Writer, targetDatabase string) error {
	if !mariaIdentifier.MatchString(targetDatabase) { return fmt.Errorf("invalid target database name") }
	reader := bufio.NewReader(source)
	for {
		line, err := reader.ReadString('\n')
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(trimmed), "USE ") {
			line = "USE " + mariaQuoteIdentifier(targetDatabase) + ";\n"
		}
		if _, writeErr := io.WriteString(destination, line); writeErr != nil { return writeErr }
		if err == io.EOF { return nil }
		if err != nil { return err }
	}
}
