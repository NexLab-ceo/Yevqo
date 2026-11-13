package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"veltrix/internal/shared"
)

func managePostgresServer(req shared.DatabaseServerAction, action string) error {
	if req.ServerID < 1 || req.Port < 1024 || req.Port > 65535 {
		return fmt.Errorf("invalid PostgreSQL server lifecycle request")
	}
	if action != "start" && action != "stop" {
		return fmt.Errorf("unsupported PostgreSQL server action")
	}
	prefix := os.Getenv("PREFIX")
	if prefix == "" {
		return fmt.Errorf("Termux PREFIX is not set")
	}
	dataDir, err := resolveTermuxPath(prefix, req.DataDirectory)
	if err != nil {
		return err
	}
	info, err := os.Lstat(dataDir)
	if err != nil {
		return fmt.Errorf("PostgreSQL data directory is unavailable")
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("PostgreSQL data directory must be a real directory")
	}
	pgctl, err := exec.LookPath("pg_ctl")
	if err != nil {
		return fmt.Errorf("pg_ctl is not installed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	statusCmd := exec.CommandContext(ctx, pgctl, "-D", dataDir, "status")
	statusOutput, statusErr := statusCmd.CombinedOutput()
	running := statusErr == nil
	if statusErr != nil && !strings.Contains(strings.ToLower(string(statusOutput)), "no server running") {
		if ctx.Err() != nil {
			return fmt.Errorf("PostgreSQL status check timed out")
		}
		return fmt.Errorf("could not inspect PostgreSQL process: %s", trimOutput(string(statusOutput)))
	}
	if action == "start" && running {
		return nil
	}
	if action == "stop" && !running {
		return nil
	}

	var args []string
	if action == "start" {
		args = []string{"-D", dataDir, "-w", "-t", "90", "start"}
	} else {
		args = []string{"-D", dataDir, "-m", "fast", "-w", "-t", "90", "stop"}
	}
	cmd := exec.CommandContext(ctx, pgctl, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("PostgreSQL %s operation timed out", action)
		}
		return fmt.Errorf("PostgreSQL %s failed: %s", action, trimOutput(string(output)))
	}
	return nil
}
