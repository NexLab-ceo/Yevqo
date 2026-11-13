package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"veltrix/internal/shared"
)

var postgresRoleName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{0,62}$`)

func managePostgresRole(req shared.ManageDatabaseRole, action string) error {
	if req.ServerID < 1 || req.Port < 1024 || req.Port > 65535 || !postgresRoleName.MatchString(req.Name) || strings.HasPrefix(strings.ToLower(req.Name), "pg_") {
		return fmt.Errorf("invalid PostgreSQL role request")
	}
	if req.Name == "postgres" {
		return fmt.Errorf("the postgres administrative role cannot be changed here")
	}
	if action != "create" && action != "drop" && action != "password" {
		return fmt.Errorf("unsupported PostgreSQL role action")
	}
	if (action == "create" || action == "password") && (len(req.Password) < 12 || len(req.Password) > 256) {
		return fmt.Errorf("role password must be 12–256 characters")
	}
	if action == "drop" && req.Password != "" {
		return fmt.Errorf("drop role request must not include a password")
	}
	prefix := os.Getenv("PREFIX")
	if prefix == "" {
		return fmt.Errorf("Termux PREFIX is not set")
	}
	psql, err := exec.LookPath("psql")
	if err != nil {
		return fmt.Errorf("psql is not installed")
	}
	identifier := `"` + strings.ReplaceAll(req.Name, `"`, `""`) + `"`
	query := "CREATE ROLE " + identifier + " LOGIN PASSWORD " + postgresSQLLiteral(req.Password) + " NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS"
	if action == "password" { query = "ALTER ROLE " + identifier + " PASSWORD " + postgresSQLLiteral(req.Password) }
	if action == "drop" {
		query = "DROP ROLE " + identifier
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, psql, "-X", "-w", "-v", "ON_ERROR_STOP=1", "-h", prefix+"/tmp", "-p", fmt.Sprint(req.Port), "-d", "postgres")
	cmd.Stdin = strings.NewReader(query)
	env := make([]string, 0, len(os.Environ())+1)
	for _, item := range os.Environ() {
		if !strings.HasPrefix(item, "PG") {
			env = append(env, item)
		}
	}
	cmd.Env = append(env, "PGCONNECT_TIMEOUT=5")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("PostgreSQL role operation timed out")
		}
		message := strings.TrimSpace(stderr.String())
		if len(message) > 240 {
			message = message[len(message)-240:]
		}
		if message == "" {
			return fmt.Errorf("PostgreSQL role operation failed")
		}
		return fmt.Errorf("PostgreSQL role operation failed: %s", message)
	}
	return nil
}

func postgresSQLLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
