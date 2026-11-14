package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"veltrix/internal/shared"
)

func listPostgresRoles(req shared.ListDatabaseRoles) ([]shared.DatabaseRole, error) {
	if req.ServerID < 1 || req.Port < 1024 || req.Port > 65535 {
		return nil, fmt.Errorf("invalid PostgreSQL role inventory request")
	}
	psql, err := exec.LookPath("psql")
	if err != nil {
		return nil, fmt.Errorf("psql is not installed")
	}
	prefix := os.Getenv("PREFIX")
	if prefix == "" {
		return nil, fmt.Errorf("Termux PREFIX is not set")
	}

	query := `SELECT COALESCE(
		json_agg(json_build_object(
			'name', rolname,
			'can_login', rolcanlogin,
			'is_superuser', rolsuper,
			'is_replication', rolreplication,
			'bypass_rls', rolbypassrls,
			'inherit', rolinherit,
			'connection_limit', rolconnlimit
		) ORDER BY rolname),
		'[]'::json
	)::text
	FROM pg_roles
	WHERE rolname !~ '^pg_';`

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, psql, "-X", "-w", "-v", "ON_ERROR_STOP=1",
		"-h", prefix+"/tmp", "-p", fmt.Sprint(req.Port), "-d", "postgres", "-tA", "-c", query)
	env := make([]string, 0, len(os.Environ())+1)
	for _, item := range os.Environ() {
		if !strings.HasPrefix(item, "PG") {
			env = append(env, item)
		}
	}
	cmd.Env = append(env, "PGCONNECT_TIMEOUT=5")
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("PostgreSQL role inventory timed out")
		}
		message := strings.TrimSpace(stderr.String())
		if len(message) > 300 {
			message = message[len(message)-300:]
		}
		if message == "" {
			return nil, fmt.Errorf("could not read PostgreSQL roles")
		}
		return nil, fmt.Errorf("could not read PostgreSQL roles: %s", message)
	}

	return parsePostgresRoles(stdout.String())
}

func parsePostgresRoles(raw string) ([]shared.DatabaseRole, error) {
	var roles []shared.DatabaseRole
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &roles); err != nil {
		return nil, fmt.Errorf("PostgreSQL returned an invalid role inventory")
	}
	if roles == nil {
		roles = []shared.DatabaseRole{}
	}
	return roles, nil
}
