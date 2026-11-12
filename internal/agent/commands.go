// Package agent — часть VELTRIX Dock, которая работает на телефонах пользователей.
// Этот файл: обработка входящих команд от сервера.
package agent

import (
	"fmt"
	"log"

	"veltrix/internal/shared"
)

type CommandHandler struct {
	cfg       *Config
	processes *ProcessManager
	tunnels   *TunnelManager
}

func NewCommandHandler(cfg *Config) *CommandHandler {
	return &CommandHandler{
		cfg:       cfg,
		processes: NewProcessManager(cfg.DataRoot),
		tunnels:   NewTunnelManager(cfg.DataRoot),
	}
}

func (h *CommandHandler) HandleCommand(cmd *shared.Command) *shared.Result {
	log.Printf("command: id=%s action=%s", cmd.Id, cmd.Action)

	switch cmd.Action {
	case "install_database_server":
		return h.handleInstallDatabaseServer(cmd)
	case "create_database":
		return h.handleCreateDatabase(cmd)
	case "create_site_backup":
		return h.handleCreateSiteBackup(cmd)
	case "get_metrics":
		return h.handleGetMetrics(cmd)
	case "create_site":
		return h.handleCreateSite(cmd)
	case "start_site":
		return h.handleStartSite(cmd)
	case "stop_site":
		return h.handleStopSite(cmd)
	case "git_pull":
		return h.handleGitPull(cmd)
	case "get_logs":
		return h.handleGetLogs(cmd)
	case "create_tunnel":
		return h.handleCreateTunnel(cmd)
	default:
		return errorResult(cmd.Id, "unknown action: "+cmd.Action)
	}
}

func (h *CommandHandler) handleInstallDatabaseServer(cmd *shared.Command) *shared.Result {
	var req shared.InstallDatabaseServer
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil { return errorResult(cmd.Id, "invalid install_database_server payload") }
	result, err := installDatabaseServer(req)
	if err != nil { return errorResult(cmd.Id, err.Error()) }
	payload, err := shared.Marshal(result)
	if err != nil { return errorResult(cmd.Id, "could not encode install result") }
	return &shared.Result{Id:cmd.Id,Ok:true,Message:"PostgreSQL is installed and running",Payload:payload}
}

func (h *CommandHandler) handleCreateSiteBackup(cmd *shared.Command) *shared.Result {
	var req shared.CreateSiteBackup
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil { return errorResult(cmd.Id, "invalid create_site_backup payload") }
	result, err := createSiteBackup(h.cfg.DataRoot, req.SiteId)
	if err != nil { return errorResult(cmd.Id, err.Error()) }
	payload, err := shared.Marshal(result)
	if err != nil { return errorResult(cmd.Id, "could not encode backup result") }
	return &shared.Result{Id:cmd.Id,Ok:true,Message:"site backup completed",Payload:payload}
}

func (h *CommandHandler) handleGetMetrics(cmd *shared.Command) *shared.Result {
	var req shared.GetMetrics
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil { return errorResult(cmd.Id, "invalid get_metrics payload") }
	metrics, err := collectMetrics(h.cfg.DataRoot)
	if err != nil { return errorResult(cmd.Id, err.Error()) }
	payload, err := shared.Marshal(metrics)
	if err != nil { return errorResult(cmd.Id, "could not encode metrics") }
	return &shared.Result{Id: cmd.Id, Ok: true, Message: "metrics collected", Payload: payload}
}

func (h *CommandHandler) handleCreateSite(cmd *shared.Command) *shared.Result {
	var req shared.CreateSite
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil { return errorResult(cmd.Id, "invalid create_site payload") }
	if err := h.processes.CreateSite(req.SiteId, int(req.Port), req.Runtime); err != nil { return errorResult(cmd.Id, err.Error()) }
	return &shared.Result{Id: cmd.Id, Ok: true, Message: "site directory created"}
}

func (h *CommandHandler) handleStartSite(cmd *shared.Command) *shared.Result {
	var req shared.StartSite
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil {
		return errorResult(cmd.Id, "invalid start_site payload")
	}

	pid, err := h.processes.StartSite(req.SiteId, req.Path, int(req.Port), req.Runtime)
	if err != nil {
		return errorResult(cmd.Id, err.Error())
	}

	resultPayload, _ := shared.Marshal(&shared.StartSiteResult{
		Pid:  uint32(pid),
		Port: req.Port,
	})

	return &shared.Result{
		Id:      cmd.Id,
		Ok:      true,
		Message: fmt.Sprintf("started on port %d (pid %d)", req.Port, pid),
		Payload: resultPayload,
	}
}

func (h *CommandHandler) handleStopSite(cmd *shared.Command) *shared.Result {
	var req shared.StopSite
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil {
		return errorResult(cmd.Id, "invalid stop_site payload")
	}

	_ = h.tunnels.StopTunnel(req.SiteId)

	if err := h.processes.StopSite(req.SiteId); err != nil {
		return errorResult(cmd.Id, err.Error())
	}

	return &shared.Result{Id: cmd.Id, Ok: true, Message: "stopped"}
}

func (h *CommandHandler) handleGitPull(cmd *shared.Command) *shared.Result {
	var req shared.GitPull
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil {
		return errorResult(cmd.Id, "invalid git_pull payload")
	}

	path, pathErr := safeSiteDirectory(h.cfg.DataRoot, req.SiteId)
	if pathErr != nil { return errorResult(cmd.Id, pathErr.Error()) }

	out, err := runGitPull(path)
	if err != nil {
		return errorResult(cmd.Id, err.Error())
	}

	return &shared.Result{Id: cmd.Id, Ok: true, Message: out}
}

func (h *CommandHandler) handleGetLogs(cmd *shared.Command) *shared.Result {
	var req shared.GetLogs
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil {
		return errorResult(cmd.Id, "invalid get_logs payload")
	}

	lines := int(req.Lines)
	if lines <= 0 {
		lines = 50
	}

	content, err := h.processes.GetLogs(req.SiteId, lines)
	if err != nil {
		return errorResult(cmd.Id, err.Error())
	}

	resultPayload, _ := shared.Marshal(&shared.LogsResult{Content: content})

	return &shared.Result{Id: cmd.Id, Ok: true, Payload: resultPayload}
}

func (h *CommandHandler) handleCreateTunnel(cmd *shared.Command) *shared.Result {
	var req shared.CreateTunnel
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil {
		return errorResult(cmd.Id, "invalid create_tunnel payload")
	}

	if !validSiteID(req.SiteId) { return errorResult(cmd.Id, "invalid site id") }
	proc, ok := h.processes.Get(req.SiteId)
	if !ok {
		return errorResult(cmd.Id, "site must be running first")
	}

	pid, err := h.tunnels.StartTunnel(req.SiteId, proc.Port, req.TunnelToken, req.Hostname)
	if err != nil {
		return errorResult(cmd.Id, err.Error())
	}

	return &shared.Result{
		Id:      cmd.Id,
		Ok:      true,
		Message: fmt.Sprintf("tunnel started (pid %d) for %s", pid, req.Hostname),
	}
}

func errorResult(cmdID, msg string) *shared.Result {
	return &shared.Result{Id: cmdID, Ok: false, Message: msg}
}

func (h *CommandHandler) handleCreateDatabase(cmd *shared.Command) *shared.Result {
	var req shared.CreateDatabase
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil { return errorResult(cmd.Id, "invalid create_database payload") }
	if err := createPostgresDatabase(req); err != nil { return errorResult(cmd.Id, err.Error()) }
	return &shared.Result{Id:cmd.Id,Ok:true,Message:"PostgreSQL database and application role created"}
}
