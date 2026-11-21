// Package agent — часть VELTRIX Dock, которая работает на телефонах пользователей.
// Этот файл: обработка входящих команд от сервера.
package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
 "os"
 "os/exec"
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
	case "prune_backups":
		return h.handlePruneBackups(cmd)
	case "create_database_backup":
		return h.handleCreateDatabaseBackup(cmd)
	case "read_database_backup_chunk":
		return h.handleReadDatabaseBackupChunk(cmd)
	case "read_site_backup_chunk":
		return h.handleReadSiteBackupChunk(cmd)
	case "restore_site_backup":
		return h.handleRestoreSiteBackup(cmd)
	case "write_backup_chunk":
		return h.handleWriteBackupChunk(cmd)
	case "restore_database_backup":
		return h.handleRestoreDatabaseBackup(cmd)
	case "test_database_connection":
		return h.handleTestDatabaseConnection(cmd)
	case "restart_agent":
  return h.handleRestartAgent(cmd)
 case "get_device_diagnostics":
  return h.handleGetDeviceDiagnostics(cmd)
 case "check_device_port":
  return h.handleCheckDevicePort(cmd)
 case "get_metrics":
		return h.handleGetMetrics(cmd)
	case "get_site_metrics":
		return h.handleGetSiteMetrics(cmd)
	case "run_scheduled_task":
		return h.handleRunScheduledTask(cmd)
	case "create_site":
		return h.handleCreateSite(cmd)
	case "analyze_site_build":
		return h.handleAnalyzeSiteBuild(cmd)
	case "build_site":
		return h.handleBuildSite(cmd)
	case "list_build_history":
		return h.handleListBuildHistory(cmd)
	case "get_build_history":
		return h.handleGetBuildHistory(cmd)
 case "list_build_artifacts":
  return h.handleListBuildArtifacts(cmd)
 case "read_build_artifact_chunk":
  return h.handleReadBuildArtifactChunk(cmd)
	case "cancel_build":
		return h.handleCancelBuild(cmd)
	case "start_site":
		return h.handleStartSite(cmd)
	case "stop_site":
		return h.handleStopSite(cmd)
	case "set_site_restart_policy":
		return h.handleSetSiteRestartPolicy(cmd)
	case "get_site_restart_policy":
		return h.handleGetSiteRestartPolicy(cmd)
	case "restart":
		return h.handleSiteLifecycle(cmd, "restart")
	case "deploy":
		return h.handleSiteLifecycle(cmd, "deploy")
	case "preview_deploy":
		return h.handlePreviewDeploy(cmd)
	case "preview_remove":
		return h.handlePreviewRemove(cmd)
	case "cancel_deployment_job":
		return h.handleCancelDeploymentJob(cmd)
	case "rebuild":
		return h.handleSiteLifecycle(cmd, "rebuild")
	case "list_site_releases":
		return h.handleListSiteReleases(cmd)
	case "rollback_site_release":
		return h.handleRollbackSiteRelease(cmd)
	case "prune_site_releases":
		return h.handlePruneSiteReleases(cmd)
	case "git_pull":
		return h.handleGitPull(cmd)
	case "get_logs":
		return h.handleGetLogs(cmd)
	case "create_tunnel":
		return h.handleCreateTunnel(cmd)
	case "stop_tunnel":
		return h.handleStopTunnel(cmd)
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
	return &shared.Result{Id:cmd.Id,Ok:true,Message:req.Engine+" is installed and running",Payload:payload}
}

func (h *CommandHandler) handlePruneBackups(cmd *shared.Command) *shared.Result {
	var req shared.PruneBackups
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil { return errorResult(cmd.Id, "invalid prune_backups payload") }
	result, err := pruneBackups(req)
	if err != nil { return errorResult(cmd.Id, err.Error()) }
	payload, err := shared.Marshal(result)
	if err != nil { return errorResult(cmd.Id, "could not encode backup retention result") }
	return &shared.Result{Id:cmd.Id,Ok:true,Message:"backup retention applied",Payload:payload}
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

func (h *CommandHandler) handleRestartAgent(cmd *shared.Command) *shared.Result {
 prefix:=os.Getenv("PREFIX");if prefix==""{prefix="/data/data/com.termux/files/usr"}
 service:="yevqo"
 if _,err:=os.Stat(prefix+"/var/service/yevqo");err!=nil {
  if _,agentErr:=os.Stat(prefix+"/var/service/yevqo-agent");agentErr==nil{service="yevqo-agent"}else{return errorResult(cmd.Id,"runit Agent service was not found")}
 }
 // Delay the supervisor restart until after the acknowledgement can be sent.
 script:="(sleep 2; sv restart "+service+") >/dev/null 2>&1 &"
 process:=exec.Command("sh","-c",script)
 if err:=process.Start();err!=nil{return errorResult(cmd.Id,"could not schedule Agent restart")}
 return &shared.Result{Id:cmd.Id,Ok:true,Message:"Agent restart scheduled"}
}

func (h *CommandHandler) handleGetDeviceDiagnostics(cmd *shared.Command) *shared.Result {
 var req shared.GetDeviceDiagnostics
 if err:=shared.Unmarshal(cmd.Payload,&req);err!=nil{return errorResult(cmd.Id,"invalid get_device_diagnostics payload")}
 result:=collectDeviceDiagnostics()
 payload,err:=shared.Marshal(result);if err!=nil{return errorResult(cmd.Id,"could not encode device diagnostics")}
 return &shared.Result{Id:cmd.Id,Ok:true,Message:"device diagnostics collected",Payload:payload}
}

func (h *CommandHandler) handleCheckDevicePort(cmd *shared.Command) *shared.Result {
 var req shared.CheckDevicePort
 if err:=shared.Unmarshal(cmd.Payload,&req);err!=nil{return errorResult(cmd.Id,"invalid check_device_port payload")}
 listening,err:=checkLocalPort(req.Port);if err!=nil{return errorResult(cmd.Id,err.Error())}
 payload,err:=shared.Marshal(map[string]interface{}{"port":req.Port,"listening":listening})
 if err!=nil{return errorResult(cmd.Id,"could not encode port check")}
 message:="port is free";if listening{message="port is listening"}
 return &shared.Result{Id:cmd.Id,Ok:true,Message:message,Payload:payload}
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
		PackageTool: req.PackageTool, InstallCommand: req.InstallCommand, BuildCommand: req.BuildCommand,
		AllowProjectCode: req.AllowProjectCode,
		Timeout: timeout, MaxLogBytes: req.MaxLogBytes,
	})
	payload, err := shared.Marshal(result)
	if err != nil { return errorResult(cmd.Id, "could not encode build result") }
	if !result.Success { return &shared.Result{Id: cmd.Id, Ok: false, Message: result.Error, Payload: payload} }
	return &shared.Result{Id: cmd.Id, Ok: true, Message: "Android local build completed", Payload: payload}
}

func (h *CommandHandler) handleListBuildHistory(cmd *shared.Command) *shared.Result {
	var req shared.ListBuildHistory
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil {
		return errorResult(cmd.Id, "invalid list_build_history payload")
	}
	records, err := ListBuildHistory(h.cfg.DataRoot, req.SiteId, req.Limit)
	if err != nil {
		return errorResult(cmd.Id, err.Error())
	}
	payload, err := shared.Marshal(map[string]interface{}{"records": records})
	if err != nil {
		return errorResult(cmd.Id, "could not encode build history")
	}
	return &shared.Result{Id: cmd.Id, Ok: true, Message: "build history loaded", Payload: payload}
}

func (h *CommandHandler) handleGetBuildHistory(cmd *shared.Command) *shared.Result {
	var req shared.GetBuildHistory
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil {
		return errorResult(cmd.Id, "invalid get_build_history payload")
	}
	record, err := GetBuildHistory(h.cfg.DataRoot, req.SiteId, req.ID)
	if err != nil {
		return errorResult(cmd.Id, err.Error())
	}
	payload, err := shared.Marshal(record)
	if err != nil {
		return errorResult(cmd.Id, "could not encode build history record")
	}
	return &shared.Result{Id: cmd.Id, Ok: true, Message: "build history record loaded", Payload: payload}
}

func (h *CommandHandler) handleListBuildArtifacts(cmd *shared.Command)*shared.Result{
 var req shared.ListBuildArtifacts;if err:=shared.Unmarshal(cmd.Payload,&req);err!=nil{return errorResult(cmd.Id,"invalid list_build_artifacts payload")}
 files,err:=ListBuildArtifacts(h.cfg.DataRoot,req.SiteId,req.BuildID);if err!=nil{return errorResult(cmd.Id,err.Error())}
 payload,err:=shared.Marshal(map[string]any{"files":files});if err!=nil{return errorResult(cmd.Id,"could not encode artifact list")}
 return &shared.Result{Id:cmd.Id,Ok:true,Message:"build artifacts listed",Payload:payload}
}
func (h *CommandHandler) handleReadBuildArtifactChunk(cmd *shared.Command)*shared.Result{
 var req shared.ReadBuildArtifactChunk;if err:=shared.Unmarshal(cmd.Payload,&req);err!=nil{return errorResult(cmd.Id,"invalid read_build_artifact_chunk payload")}
 data,total,eof,err:=ReadBuildArtifactChunk(h.cfg.DataRoot,req.SiteId,req.BuildID,req.Path,req.Offset,req.Limit);if err!=nil{return errorResult(cmd.Id,err.Error())}
 payload,err:=shared.Marshal(map[string]any{"data":data,"total_bytes":total,"eof":eof,"offset":req.Offset});if err!=nil{return errorResult(cmd.Id,"could not encode artifact chunk")}
 return &shared.Result{Id:cmd.Id,Ok:true,Message:"artifact chunk loaded",Payload:payload}
}

func (h *CommandHandler) handleCancelBuild(cmd *shared.Command) *shared.Result {
	if err := h.processes.CancelBuild(); err != nil {
		return errorResult(cmd.Id, err.Error())
	}
	return &shared.Result{Id: cmd.Id, Ok: true, Message: "build cancellation requested"}
}

func (h *CommandHandler) handleCreateSite(cmd *shared.Command) *shared.Result {
	var req shared.CreateSite
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil { return errorResult(cmd.Id, "invalid create_site payload") }
	if err := h.processes.CreateSiteFromTemplate(req.SiteId, int(req.Port), req.TemplateID, req.Runtime); err != nil { return errorResult(cmd.Id, err.Error()) }
	return &shared.Result{Id: cmd.Id, Ok: true, Message: "site directory created"}
}

func (h *CommandHandler) handleStartSite(cmd *shared.Command) *shared.Result {
	var req shared.StartSite
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil {
		return errorResult(cmd.Id, "invalid start_site payload")
	}

	pid, err := h.processes.StartSiteWithEnvAndCommand(req.SiteId, req.Path, int(req.Port), req.Runtime, req.Env, req.StartCommand)
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

	if err := h.processes.StopSite(req.SiteId); err != nil {
		return errorResult(cmd.Id, err.Error())
	}

	return &shared.Result{Id: cmd.Id, Ok: true, Message: "stopped"}
}

func (h *CommandHandler) handleSiteLifecycle(cmd *shared.Command, action string) *shared.Result {
 var req shared.SiteAction
 if err := shared.Unmarshal(cmd.Payload, &req); err != nil { return errorResult(cmd.Id, "invalid site action payload") }
 if action == "restart" {
  pid, err := h.processes.RestartSiteWithEnv(req.SiteId, req.Path, int(req.Port), req.Runtime, req.Env)
  if err != nil { return errorResult(cmd.Id, err.Error()) }
  payload, _ := shared.Marshal(&shared.StartSiteResult{Pid:uint32(pid), Port:req.Port})
  return &shared.Result{Id:cmd.Id, Ok:true, Message:fmt.Sprintf("restart completed (pid %d)", pid), Payload:payload}
 }
 if action != "deploy" && action != "rebuild" { return errorResult(cmd.Id, "unsupported site lifecycle action") }
 jobID, jobCtx, err := h.processes.BeginDeploymentJob(req.SiteId)
 if err != nil { return errorResult(cmd.Id, err.Error()) }
 pid, output, commitHash, commitDate, deployErr := h.processes.DeployGitSiteWithJob(jobCtx, jobID, req, action == "rebuild")
 if deployErr != nil {
  state := "failed"
  if errors.Is(jobCtx.Err(), context.Canceled) { state = "cancelled" }
  h.processes.FinishDeploymentJob(req.SiteId, jobID, state, deployErr.Error())
  return deploymentErrorResult(cmd.Id, deployErr)
 }
 h.processes.FinishDeploymentJob(req.SiteId, jobID, "healthy", "Deployment completed")
 payload, _ := shared.Marshal(&shared.StartSiteResult{Pid:uint32(pid), Port:req.Port, CommitHash:commitHash, CommitDate:commitDate})
 return &shared.Result{Id:cmd.Id, Ok:true, Message:fmt.Sprintf("%s completed (pid %d): %s", action, pid, output), Payload:payload}
}

func (h *CommandHandler) handlePreviewDeploy(cmd *shared.Command) *shared.Result {
	var req shared.PreviewDeploy
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil {
		return errorResult(cmd.Id, "invalid preview_deploy payload")
	}
	if !req.AllowProjectCode {
		return errorResult(cmd.Id, "preview blocked: explicit allow_project_code=true consent is required")
	}
	pid, output, commit, date, err := h.processes.DeployPreview(req)
	if err != nil {
		_ = h.processes.RemovePreview(req.PreviewID)
		return errorResult(cmd.Id, err.Error())
	}
	payload, err := shared.Marshal(&shared.StartSiteResult{Pid: uint32(pid), Port: req.Port, CommitHash: commit, CommitDate: date})
	if err != nil {
		_ = h.processes.RemovePreview(req.PreviewID)
		return errorResult(cmd.Id, "could not encode preview result")
	}
	return &shared.Result{Id: cmd.Id, Ok: true, Message: fmt.Sprintf("preview ready (pid %d): %s", pid, output), Payload: payload}
}

func (h *CommandHandler) handlePreviewRemove(cmd *shared.Command) *shared.Result {
	var req shared.PreviewRemove
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil {
		return errorResult(cmd.Id, "invalid preview_remove payload")
	}
	if err := h.processes.RemovePreview(req.PreviewID); err != nil {
		return errorResult(cmd.Id, err.Error())
	}
	return &shared.Result{Id: cmd.Id, Ok: true, Message: "preview removed"}
}

func (h *CommandHandler) handleCancelDeploymentJob(cmd *shared.Command) *shared.Result {
 var req shared.DeploymentJobAction
 if err := shared.Unmarshal(cmd.Payload, &req); err != nil { return errorResult(cmd.Id, "invalid cancel_deployment_job payload") }
 if err := h.processes.CancelDeploymentJob(req.SiteId, req.JobId); err != nil { return errorResult(cmd.Id, err.Error()) }
 return &shared.Result{Id:cmd.Id, Ok:true, Message:"deployment cancellation requested"}
}


func (h *CommandHandler) handleListSiteReleases(cmd *shared.Command) *shared.Result {
	var req shared.ListSiteReleases
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil { return errorResult(cmd.Id, "invalid list_site_releases payload") }
	releases, err := h.processes.ListSiteReleases(req.SiteId, req.Runtime)
	if err != nil { return errorResult(cmd.Id, err.Error()) }
	payload, err := shared.Marshal(&shared.SiteReleasesResult{Releases:releases})
	if err != nil { return errorResult(cmd.Id, "could not encode release history") }
	return &shared.Result{Id:cmd.Id, Ok:true, Message:"release history loaded", Payload:payload}
}

func (h *CommandHandler) handleRollbackSiteRelease(cmd *shared.Command) *shared.Result {
	var req shared.RollbackSiteRelease
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil { return errorResult(cmd.Id, "invalid rollback_site_release payload") }
	pid, err := h.processes.RollbackSiteRelease(req.SiteId, req.Commit, req.RootDir, req.Runtime, int(req.Port), req.StartCommand, req.Env)
	if err != nil { return errorResult(cmd.Id, err.Error()) }
	payload, _ := shared.Marshal(&shared.StartSiteResult{Pid:uint32(pid), Port:req.Port, CommitHash:req.Commit})
	return &shared.Result{Id:cmd.Id, Ok:true, Message:fmt.Sprintf("rollback completed: release %s is active (pid %d)", req.Commit, pid), Payload:payload}
}

func (h *CommandHandler) handleGitPull(cmd *shared.Command) *shared.Result {
	var req shared.GitPull
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil {
		return errorResult(cmd.Id, "invalid git_pull payload")
	}

	out, err := h.processes.GitPullSite(req.SiteId)
	if err != nil {
		return errorResult(cmd.Id, err.Error())
	}

	return &shared.Result{Id: cmd.Id, Ok: true, Message: out}
}

func (h *CommandHandler) handleGetSiteMetrics(cmd *shared.Command) *shared.Result {
	var req shared.GetSiteMetrics
	if err:=shared.Unmarshal(cmd.Payload,&req);err!=nil{return errorResult(cmd.Id,"invalid get_site_metrics payload")}
	metrics,err:=h.processes.GetSiteMetrics(req.SiteId)
	if err!=nil{return errorResult(cmd.Id,err.Error())}
	payload,err:=shared.Marshal(metrics);if err!=nil{return errorResult(cmd.Id,"could not encode site metrics")}
	return &shared.Result{Id:cmd.Id,Ok:true,Message:"site process metrics collected",Payload:payload}
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

func (h *CommandHandler) handleStopTunnel(cmd *shared.Command) *shared.Result {
 var req shared.StopTunnel
 if err:=shared.Unmarshal(cmd.Payload,&req);err!=nil{return errorResult(cmd.Id,"invalid stop_tunnel payload")}
 if !validSiteID(req.SiteId){return errorResult(cmd.Id,"invalid site id")}
 if err:=h.tunnels.StopTunnel(req.SiteId);err!=nil{return errorResult(cmd.Id,err.Error())}
 return &shared.Result{Id:cmd.Id,Ok:true,Message:"Cloudflare Tunnel stopped"}
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

	if req.Port != 0 && int(req.Port) != proc.Port { return errorResult(cmd.Id, "site port mismatch") }
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

func deploymentErrorResult(cmdID string, err error) *shared.Result {
	failure := classifyDeploymentFailure(err)
	return &shared.Result{Id:cmdID, Ok:false, Message:err.Error(), ErrorCode:failure.Code, Retryable:failure.Retryable}
}

func (h *CommandHandler) handleCreateDatabase(cmd *shared.Command) *shared.Result {
	var req shared.CreateDatabase
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil { return errorResult(cmd.Id, "invalid create_database payload") }
	var err error; if req.Engine == "mariadb" || req.Engine == "mysql" { err = createMariaDBDatabase(req) } else { err = createPostgresDatabase(req) }; if err != nil { return errorResult(cmd.Id, err.Error()) }
	return &shared.Result{Id:cmd.Id,Ok:true,Message:req.Engine+" database and application user created"}
}

func (h *CommandHandler) handleCreateDatabaseBackup(cmd *shared.Command) *shared.Result {
	var req shared.CreateDatabaseBackup
	if err:=shared.Unmarshal(cmd.Payload,&req);err!=nil{return errorResult(cmd.Id,"invalid create_database_backup payload")}
	var result *shared.DatabaseBackupResult; var err error; if req.Engine == "mariadb" || req.Engine == "mysql" { result,err=createMariaDBBackup(req) } else { result,err=createDatabaseBackup(h.cfg.DataRoot,req) }
	if err!=nil{return errorResult(cmd.Id,err.Error())}
	payload,err:=shared.Marshal(result);if err!=nil{return errorResult(cmd.Id,"could not encode database backup result")}
	return &shared.Result{Id:cmd.Id,Ok:true,Message:req.Engine+" database backup completed",Payload:payload}
}

func (h *CommandHandler) handleRestoreDatabaseBackup(cmd *shared.Command) *shared.Result {
	var req shared.RestoreDatabaseBackup
	if err:=shared.Unmarshal(cmd.Payload,&req);err!=nil{return errorResult(cmd.Id,"invalid restore_database_backup payload")}
	var err error; if req.Database.Engine == "mariadb" || req.Database.Engine == "mysql" { err=restoreMariaDBBackup(req) } else { err=restorePostgresDatabase(req) }; if err!=nil{return errorResult(cmd.Id,err.Error())}
	return &shared.Result{Id:cmd.Id,Ok:true,Message:req.Database.Engine+" backup restored into a new database"}
}

func (h *CommandHandler) handleTestDatabaseConnection(cmd *shared.Command) *shared.Result {
	var req shared.TestDatabaseConnection
	if err:=shared.Unmarshal(cmd.Payload,&req);err!=nil{return errorResult(cmd.Id,"invalid test_database_connection payload")}
	var err error; if req.Engine == "mariadb" || req.Engine == "mysql" { err=testMariaDBConnection(req) } else { err=testPostgresConnection(req) }; if err!=nil{return errorResult(cmd.Id,err.Error())}
	return &shared.Result{Id:cmd.Id,Ok:true,Message:req.Engine+" connection test succeeded"}
}


func (h *CommandHandler) handleManageDatabaseRole(cmd *shared.Command, action string) *shared.Result {
	var req shared.ManageDatabaseRole
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil {
		return errorResult(cmd.Id, "invalid PostgreSQL role payload")
	}
	var err error; if req.Engine == "mariadb" || req.Engine == "mysql" { err=manageMariaDBRole(req,action) } else { err=managePostgresRole(req,action) }; if err != nil {
		return errorResult(cmd.Id, err.Error())
	}
	return &shared.Result{Id: cmd.Id, Ok: true, Message: req.Engine+" user " + action + " completed"}
}

func (h *CommandHandler) handleListDatabaseRoles(cmd *shared.Command) *shared.Result {
	var req shared.ListDatabaseRoles
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil {
		return errorResult(cmd.Id, "invalid list_database_roles payload")
	}
	var roles []shared.DatabaseRole; var err error; if req.Engine == "mariadb" || req.Engine == "mysql" { roles,err=listMariaDBRoles(req) } else { roles,err=listPostgresRoles(req) }
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
	var err error; if req.Engine == "mariadb" || req.Engine == "mysql" { err=manageMariaDBServer(req,action) } else { err=managePostgresServer(req,action) }; if err != nil {
		return errorResult(cmd.Id, err.Error())
	}
	return &shared.Result{Id: cmd.Id, Ok: true, Message: req.Engine+" server " + action + " completed"}
}

 
func (h *CommandHandler) handleInstallCloudflared(cmd *shared.Command) *shared.Result {
 path,err:=h.tunnels.InstallCloudflared()
 if err!=nil{return errorResult(cmd.Id,err.Error())}
 payload,_:=shared.Marshal(map[string]string{"path":path})
 return &shared.Result{Id:cmd.Id,Ok:true,Message:"cloudflared installed for Termux user",Payload:payload}
}

func (h *CommandHandler) handlePruneSiteReleases(cmd *shared.Command) *shared.Result {
	var req shared.PruneSiteReleases
	if err := shared.Unmarshal(cmd.Payload, &req); err != nil {
		return errorResult(cmd.Id, "invalid prune_site_releases payload")
	}
	removed, err := h.processes.PruneSiteReleases(req.SiteId, req.Keep)
	if err != nil {
		return errorResult(cmd.Id, err.Error())
	}
	payload, err := shared.Marshal(&shared.PruneSiteReleasesResult{Removed: removed})
	if err != nil {
		return errorResult(cmd.Id, "could not encode release pruning result")
	}
	return &shared.Result{Id: cmd.Id, Ok: true, Message: fmt.Sprintf("removed %d old release(s)", len(removed)), Payload: payload}
}


func (h *CommandHandler) handleSetSiteRestartPolicy(cmd *shared.Command) *shared.Result {
 var req shared.SetSiteRestartPolicy
 if err := shared.Unmarshal(cmd.Payload, &req); err != nil { return errorResult(cmd.Id, "invalid set_site_restart_policy payload") }
 if err := h.processes.SetRestartPolicy(req.SiteId, req.Policy); err != nil { return errorResult(cmd.Id, err.Error()) }
 return &shared.Result{Id: cmd.Id, Ok: true, Message: "restart policy saved: " + req.Policy}
}


func (h *CommandHandler) handleGetSiteRestartPolicy(cmd *shared.Command) *shared.Result {
 var req shared.GetSiteRestartPolicy
 if err := shared.Unmarshal(cmd.Payload, &req); err != nil { return errorResult(cmd.Id, "invalid get_site_restart_policy payload") }
 policy, err := h.processes.GetRestartPolicy(req.SiteId)
 if err != nil { return errorResult(cmd.Id, err.Error()) }
 payload, err := shared.Marshal(map[string]string{"policy": policy})
 if err != nil { return errorResult(cmd.Id, "could not encode restart policy") }
 return &shared.Result{Id: cmd.Id, Ok: true, Message: "restart policy loaded", Payload: payload}
}



func (h *CommandHandler) handleRunScheduledTask(cmd *shared.Command) *shared.Result {
	var req shared.RunScheduledTask
	if err:=shared.Unmarshal(cmd.Payload,&req);err!=nil{return errorResult(cmd.Id,"invalid run_scheduled_task payload")}
	result,err:=h.processes.RunScheduledTask(req);if err!=nil{return errorResult(cmd.Id,err.Error())}
	payload,err:=shared.Marshal(result);if err!=nil{return errorResult(cmd.Id,"could not encode scheduled task result")}
	message:="scheduled task completed";if result.ExitCode!=0{message="scheduled task failed"}
	return &shared.Result{Id:cmd.Id,Ok:true,Message:message,Payload:payload}
}
