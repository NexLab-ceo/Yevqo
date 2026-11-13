package control

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/go-sql-driver/mysql"
)

var mysqlDatabaseNamePattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// ensureMySQLDatabase creates the Control Plane database if it does not exist.
// The MariaDB CLI is used because Termux's local root account is commonly
// authenticated through the unix_socket plugin, which database drivers do not
// implement. The identifier is parsed from the application DSN and strictly
// validated before it is interpolated into SQL.
func ensureMySQLDatabase(ctx context.Context, appDSN string) error {
	cfg, err := mysql.ParseDSN(appDSN)
	if err != nil {
		return fmt.Errorf("parse application MySQL DSN: %w", err)
	}
	name := cfg.DBName
	if !mysqlDatabaseNamePattern.MatchString(name) {
		return fmt.Errorf("invalid Control Plane database name %q", name)
	}

	client, err := exec.LookPath("mariadb")
	if err != nil {
		client, err = exec.LookPath("mysql")
	}
	if err != nil {
		return fmt.Errorf("MariaDB client is required to bootstrap the Control Plane database: %w", err)
	}

	query := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS \`%s\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", name)
	cmd := exec.CommandContext(ctx, client, "-u", "root", "-e", query)
	output, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			return fmt.Errorf("create Control Plane database %q: %w", name, err)
		}
		return fmt.Errorf("create Control Plane database %q: %w: %s", name, err, detail)
	}
	return nil
}

