// Package agent — часть VELTRIX Dock, которая работает на телефонах пользователей.
// Этот файл: обработка входящих команд от сервера.
package agent

import (
	"fmt"
	"log"
	"time"

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
	case "start_database_server":
		return h.handleDatabaseServerAction(cmd, "start")
	case "stop_database_server":
		return h.handleDatabaseServerAction(cmd, "stop")
	case "create_database":
		return h.handleCreateDatabase(cmd)
	case "list_database_roles":
		return h.handleListDatabaseRoles(cmd)
	case "create_database_role":
		return h.handleManageDatabaseRole(cmd, "create")
	case "drop_database_role":
		return h.handleManageDatabaseRole(cmd, "drop")
	case "password_database_role":
		return h.handleManageDatabaseRole(cmd, "password")
	case "manage_database_privilege":
		return h.handleManageDatabasePrivilege(cmd)
	case "get_database_stats":
		return h.handleGetDatabaseStats(cmd)
	case "create_site_backup":
		return h.handleCreateSiteBackup(cmd)
	case "create_database_backup":
		return h.handleCreateDatabaseBackup(cmd)
	case "read_database_backup_chunk":
		return h.handleReadDatabaseBackupChunk(cmd)
	case "restore_database_backup":
		return h.handleRestoreDatabaseBackup(cmd)
	case "test_database_connection":
		return h.handleTestDatabaseConnection(cmd)
	case "get_metrics":
		return h.handleGetMetrics(cmd)
	case "create_site":
		return h.handleCreateSite(cmd)
	case "analyze_site_build":
		return h.handleAnalyzeSiteBuild(cmd)
	case "build_site":
		return h.handleBuildSite(cmd)
	case "start_site":
		return h.handleStartSite(cmd)
	case "stop_site":
		return h.handleStopSite(cmd)
	case "restart":
		return h.handleSiteLifecycle(cmd, "restart")
	case "deploy":
		return h.handleSiteLifecycle(cmd, "deploy")
	case "rebuild":
		return h.handleSiteLifecycle(cmd, "rebuild")
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

func (h *CommandHandler) handleAnalyzeSiteBuild(cmd *shared.Command) *shared.Result {
	var req shared.AnalyzeSiteBuild
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil {
		return errorResult(cmd.Id, "invalid analyze_site_build payload")
	}
	plan, err := h.processes.AnalyzeSiteBuild(req.SiteId, req.RootDir)
	if err != nil {
		return errorResult(cmd.Id, err.Error())
	}
	payload, err := shared.Marshal(plan)
	if err != nil {
		return errorResult(cmd.Id, "could not encode build analysis")
	}
	return &shared.Result{Id: cmd.Id, Ok: true, Message: "project analysis completed", Payload: payload}
}

func (h *CommandHandler) handleBuildSite(cmd *shared.Command) *shared.Result {
	var req shared.AndroidBuildSite
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil {
		return errorResult(cmd.Id, "invalid build_site payload")
	}
	timeout := time.Duration(req.TimeoutSeconds) * time.Second
	result := h.processes.BuildSite(AndroidBuildRequest{
		SiteID: req.SiteId, RootDir: req.RootDir, Stack: req.Stack,
		PackageTool: req.PackageTool, AllowProjectCode: req.AllowProjectCode,
		Timeout: timeout, MaxLogBytes: req.MaxLogBytes,
	})
	payload, err := shared.Marshal(result)
	if err != nil { return errorResult(cmd.Id, "could not encode build result") }
	if !result.Success { return &shared.Result{Id: cmd.Id, Ok: false, Message: result.Error, Payload: payload} }
	return &shared.Result{Id: cmd.Id, Ok: true, Message: "Android local build completed", Payload: payload}
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

	pid, err := h.processes.StartSiteWithEnv(req.SiteId, req.Path, int(req.Port), req.Runtime, req.Env)
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

func (h *CommandHandler) handleSiteLifecycle(cmd *shared.Command, action string) *shared.Result {
	var req shared.SiteAction
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil { return errorResult(cmd.Id, "invalid site action payload") }
	var pid int
	var output, commitHash, commitDate string
	var err error
	switch action {
	case "restart":
		pid, err = h.processes.RestartSite(req.SiteId, req.Path, int(req.Port), req.Runtime)
	case "deploy":
		pid, output, commitHash, commitDate, err = h.processes.DeployGitSite(req, false)
	case "rebuild":
		pid, output, commitHash, commitDate, err = h.processes.DeployGitSite(req, true)
	}
	if err != nil { return errorResult(cmd.Id, err.Error()) }
	payload, _ := shared.Marshal(&shared.StartSiteResult{Pid: uint32(pid), Port: req.Port, CommitHash: commitHash, CommitDate: commitDate})
	return &shared.Result{Id: cmd.Id, Ok: true, Message: fmt.Sprintf("%s completed (pid %d): %s", action, pid, output), Payload: payload}
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

	if req.Port != 0 && int(req.Port) != proc.Port { return errorResult(cmd.Id, "site port mismatch") }\n\tpid, err := h.tunnels.StartTunnel(req.SiteId, proc.Port, req.TunnelToken, req.Hostname)
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

func (h *CommandHandler) handleCreateDatabaseBackup(cmd *shared.Command) *shared.Result {
	var req shared.CreateDatabaseBackup
	if err:=shared.Unmarshal(cmd.Payload,&req);err!=nil{return errorResult(cmd.Id,"invalid create_database_backup payload")}
	result,err:=createDatabaseBackup(h.cfg.DataRoot,req)
	if err!=nil{return errorResult(cmd.Id,err.Error())}
	payload,err:=shared.Marshal(result);if err!=nil{return errorResult(cmd.Id,"could not encode database backup result")}
	return &shared.Result{Id:cmd.Id,Ok:true,Message:"PostgreSQL database backup completed",Payload:payload}
}

func (h *CommandHandler) handleRestoreDatabaseBackup(cmd *shared.Command) *shared.Result {
	var req shared.RestoreDatabaseBackup
	if err:=shared.Unmarshal(cmd.Payload,&req);err!=nil{return errorResult(cmd.Id,"invalid restore_database_backup payload")}
	if err:=restorePostgresDatabase(req);err!=nil{return errorResult(cmd.Id,err.Error())}
	return &shared.Result{Id:cmd.Id,Ok:true,Message:"PostgreSQL backup restored into a new database"}
}

func (h *CommandHandler) handleTestDatabaseConnection(cmd *shared.Command) *shared.Result {
	var req shared.TestDatabaseConnection
	if err:=shared.Unmarshal(cmd.Payload,&req);err!=nil{return errorResult(cmd.Id,"invalid test_database_connection payload")}
	if err:=testPostgresConnection(req);err!=nil{return errorResult(cmd.Id,err.Error())}
	return &shared.Result{Id:cmd.Id,Ok:true,Message:"PostgreSQL connection test succeeded"}
}


func (h *CommandHandler) handleManageDatabaseRole(cmd *shared.Command, action string) *shared.Result {
	var req shared.ManageDatabaseRole
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil {
		return errorResult(cmd.Id, "invalid PostgreSQL role payload")
	}
	if err := managePostgresRole(req, action); err != nil {
		return errorResult(cmd.Id, err.Error())
	}
	return &shared.Result{Id: cmd.Id, Ok: true, Message: "PostgreSQL role " + action + " completed"}
}

func (h *CommandHandler) handleListDatabaseRoles(cmd *shared.Command) *shared.Result {
	var req shared.ListDatabaseRoles
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil {
		return errorResult(cmd.Id, "invalid list_database_roles payload")
	}
	roles, err := listPostgresRoles(req)
	if err != nil {
		return errorResult(cmd.Id, err.Error())
	}
	payload, err := shared.Marshal(&shared.DatabaseRolesResult{Roles: roles})
	if err != nil {
		return errorResult(cmd.Id, "could not encode PostgreSQL role inventory")
	}
	return &shared.Result{Id: cmd.Id, Ok: true, Message: "PostgreSQL role inventory loaded", Payload: payload}
}


func (h *CommandHandler) handleDatabaseServerAction(cmd *shared.Command, action string) *shared.Result {
	var req shared.DatabaseServerAction
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil {
		return errorResult(cmd.Id, "invalid database server action payload")
	}
	if err := managePostgresServer(req, action); err != nil {
		return errorResult(cmd.Id, err.Error())
	}
	return &shared.Result{Id: cmd.Id, Ok: true, Message: "PostgreSQL server " + action + " completed"}
}

 
func (h *CommandHandler) handleInstallCloudflared(cmd *shared.Command) *shared.Result {
 path,err:=h.tunnels.InstallCloudflared()
 if err!=nil{return errorResult(cmd.Id,err.Error())}
 payload,_:=shared.Marshal(map[string]string{"path":path})
 return &shared.Result{Id:cmd.Id,Ok:true,Message:"cloudflared installed for Termux user",Payload:payload}
}
