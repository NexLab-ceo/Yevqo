package control

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"veltrix/internal/shared"
)

func (s *Server) handleControlPlaneBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, err := os.Stat("/sdcard"); err != nil {
		apiError(w, shared.ErrInternal("shared storage /sdcard is not available"))
		return
	}
	cfg, err := mysql.ParseDSN(s.cfg.MySQLDSN)
	if err != nil {
		apiError(w, shared.ErrInternal("invalid database connection configuration"))
		return
	}
	host, port := "127.0.0.1", "3306"
	if cfg.Net == "tcp" {
		if h, p, splitErr := net.SplitHostPort(cfg.Addr); splitErr == nil {
			host, port = h, p
		}
	}
	dir := filepath.Join(s.cfg.DataRoot, "backups")
	if err := os.MkdirAll(dir, 0700); err != nil {
		apiError(w, shared.ErrInternal("could not prepare backup directory"))
		return
	}
	cnfFile, err := os.CreateTemp("", "veltrix-mysqldump-*.cnf")
	if err != nil {
		apiError(w, shared.ErrInternal("could not prepare secure database credentials"))
		return
	}
	cnf := cnfFile.Name()
	defer os.Remove(cnf)
	cnfContent := fmt.Sprintf("[client]\\nuser=%s\\npassword=%s\\nhost=%s\\nport=%s\\nprotocol=tcp\\n",
		cnfEscape(cfg.User), cnfEscape(cfg.Passwd), cnfEscape(host), cnfEscape(port))
	if err := cnfFile.Chmod(0600); err != nil {
		_ = cnfFile.Close()
		apiError(w, shared.ErrInternal("could not secure database credentials"))
		return
	}
	if _, err := cnfFile.WriteString(cnfContent); err != nil {
		_ = cnfFile.Close()
		apiError(w, shared.ErrInternal("could not prepare secure database credentials"))
		return
	}
	if err := cnfFile.Close(); err != nil {
		apiError(w, shared.ErrInternal("could not close database credential file"))
		return
	}
	dumpBin, err := exec.LookPath("mariadb-dump")
	if err != nil {
		dumpBin, err = exec.LookPath("mysqldump")
	}
	if err != nil {
		apiError(w, shared.ErrInternal("mariadb-dump or mysqldump is not installed"))
		return
	}

	name := "veltrix-control-" + time.Now().UTC().Format("20060102-150405") + ".sql.gz"
	path := filepath.Join(dir, name)
	out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		apiError(w, shared.ErrInternal("could not create backup file"))
		return
	}
	ok := false
	defer func() {
		_ = out.Close()
		if !ok { _ = os.Remove(path) }
	}()

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, dumpBin, "--defaults-extra-file="+cnf, "--single-transaction", "--routines", "--triggers", "--databases", cfg.DBName)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		apiError(w, shared.ErrInternal("could not start database dump"))
		return
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		apiError(w, shared.ErrInternal("could not start database dump"))
		return
	}
	gz := gzip.NewWriter(out)
	_, copyErr := io.Copy(gz, stdout)
	closeErr := gz.Close()
	waitErr := cmd.Wait()
	if copyErr != nil || closeErr != nil || waitErr != nil {
		apiError(w, shared.ErrInternal("database backup failed: " + strings.TrimSpace(stderr.String())))
		return
	}
	if err := out.Sync(); err != nil {
		apiError(w, shared.ErrInternal("could not flush backup file"))
		return
	}
	info, err := out.Stat()
	if err != nil {
		apiError(w, shared.ErrInternal("could not verify backup file"))
		return
	}
	ok = true
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true, "file_name": name, "path": path, "size_bytes": info.Size(),
		"created_at": time.Now().UTC(), "message": "Control Plane database backup created",
	})
}

func cnfEscape(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	value = strings.ReplaceAll(value, "\n", "")
	value = strings.ReplaceAll(value, "\r", "")
	return "\"" + value + "\""
}
