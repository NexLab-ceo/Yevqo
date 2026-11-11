// Package agent — часть VELTRIX Dock, которая работает на телефонах пользователей.
// Этот файл: обработка входящих команд от сервера.
package agent

import (
	"fmt"
	"log"

	"google.golang.org/protobuf/proto"
	pb "veltrix/internal/shared"
)

// CommandHandler обрабатывает команды от сервера.
type CommandHandler struct {
	cfg        *Config
	processes  *ProcessManager
	tunnels    *TunnelManager
}

// NewCommandHandler создаёт обработчик.
func NewCommandHandler(cfg *Config) *CommandHandler {
	return &CommandHandler{
		cfg:       cfg,
		processes: NewProcessManager(cfg.DataRoot),
		tunnels:   NewTunnelManager(cfg.DataRoot),
	}
}

// HandleCommand — главная точка входа.
// Возвращает Result, который агент отправит серверу.
func (h *CommandHandler) HandleCommand(cmd *pb.Command) *pb.Result {
	log.Printf("command: id=%s action=%s", cmd.Id, cmd.Action)

	switch cmd.Action {
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
		return &pb.Result{
			Id:      cmd.Id,
			Ok:      false,
			Message: "unknown action: " + cmd.Action,
		}
	}
}

// ============================================================
// start_site
// ============================================================

func (h *CommandHandler) handleStartSite(cmd *pb.Command) *pb.Result {
	var req pb.StartSite
	if err := proto.Unmarshal(cmd.Payload, &req); err != nil {
		return errorResult(cmd.Id, "invalid start_site payload")
	}

	pid, err := h.processes.StartSite(
		req.SiteId,
		req.Path,
		int(req.Port),
		req.Runtime,
	)
	if err != nil {
		return errorResult(cmd.Id, err.Error())
	}

	// Сериализуем payload результата.
	resultPayload, _ := proto.Marshal(&pb.StartSiteResult{
		Pid:  uint32(pid),
		Port: req.Port,
	})

	return &pb.Result{
		Id:      cmd.Id,
		Ok:      true,
		Message: fmt.Sprintf("started on port %d (pid %d)", req.Port, pid),
		Payload: resultPayload,
	}
}

// ============================================================
// stop_site
// ============================================================

func (h *CommandHandler) handleStopSite(cmd *pb.Command) *pb.Result {
	var req pb.StopSite
	if err := proto.Unmarshal(cmd.Payload, &req); err != nil {
		return errorResult(cmd.Id, "invalid stop_site payload")
	}

	// Останавливаем туннель (если был).
	_ = h.tunnels.StopTunnel(req.SiteId)

	// Останавливаем сам сайт.
	if err := h.processes.StopSite(req.SiteId); err != nil {
		return errorResult(cmd.Id, err.Error())
	}

	return &pb.Result{
		Id:      cmd.Id,
		Ok:      true,
		Message: "stopped",
	}
}

// ============================================================
// git_pull
// ============================================================

func (h *CommandHandler) handleGitPull(cmd *pb.Command) *pb.Result {
	var req pb.GitPull
	if err := proto.Unmarshal(cmd.Payload, &req); err != nil {
		return errorResult(cmd.Id, "invalid git_pull payload")
	}

	// Путь к папке сайта.
	path := h.cfg.DataRoot + "/sites/" + req.SiteId

	out, err := runGitPull(path)
	if err != nil {
		return errorResult(cmd.Id, err.Error())
	}

	return &pb.Result{
		Id:      cmd.Id,
		Ok:      true,
		Message: out,
	}
}

// ============================================================
// get_logs
// ============================================================

func (h *CommandHandler) handleGetLogs(cmd *pb.Command) *pb.Result {
	var req pb.GetLogs
	if err := proto.Unmarshal(cmd.Payload, &req); err != nil {
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

	resultPayload, _ := proto.Marshal(&pb.LogsResult{
		Content: content,
	})

	return &pb.Result{
		Id:      cmd.Id,
		Ok:      true,
		Payload: resultPayload,
	}
}

// ============================================================
// create_tunnel
// ============================================================

func (h *CommandHandler) handleCreateTunnel(cmd *pb.Command) *pb.Result {
	var req pb.CreateTunnel
	if err := proto.Unmarshal(cmd.Payload, &req); err != nil {
		return errorResult(cmd.Id, "invalid create_tunnel payload")
	}

	// Находим порт сайта (из запущенных процессов).
	proc, ok := h.processes.Get(req.SiteId)
	if !ok {
		return errorResult(cmd.Id, "site must be running first")
	}

	pid, err := h.tunnels.StartTunnel(
		req.SiteId,
		proc.Port,
		req.TunnelToken,
		req.Hostname,
	)
	if err != nil {
		return errorResult(cmd.Id, err.Error())
	}

	return &pb.Result{
		Id:      cmd.Id,
		Ok:      true,
		Message: fmt.Sprintf("tunnel started (pid %d) for %s", pid, req.Hostname),
	}
}

// ============================================================
// УТИЛИТЫ
// ============================================================

// errorResult создаёт Result с ошибкой.
func errorResult(cmdID, msg string) *pb.Result {
	return &pb.Result{
		Id:      cmdID,
		Ok:      false,
		Message: msg,
	}
}
