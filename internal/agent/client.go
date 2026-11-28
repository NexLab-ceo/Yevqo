// Package agent — часть VELTRIX Dock, которая работает на телефонах пользователей.
// Этот файл: WebSocket-клиент (подключение к серверу, приём команд, отправка ответов).
package agent

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"regexp"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"time"
	"strings"

	"github.com/gorilla/websocket"
	"veltrix/internal/shared"
)

func detectAgentCapabilities() []string {
	// Report only capabilities that are actually installed on this device.
	// Docker is intentionally excluded: Termux is not treated as a Docker host.
	checks := []struct{ name, binary string }{
		{"git", "git"}, {"php", "php"}, {"nodejs", "node"},
		{"npm", "npm"}, {"go", "go"}, {"composer", "composer"},
		{"python", "python"}, {"python3", "python3"},
	}
	capabilities := make([]string, 0, len(checks)+1)
	capabilities = append(capabilities, "android-local-process")
	for _, check := range checks {
		if _, err := exec.LookPath(check.binary); err == nil {
			capabilities = append(capabilities, check.name)
		}
	}
	return capabilities
}

const (
	Version        = "1.0.0"
	ReconnectDelay = 5 * time.Second
	WriteWait      = 10 * time.Second
	PongWait       = 60 * time.Second
	PingPeriod     = 30 * time.Second
	MaxMessageSize = 1 << 20
)

// Client — WebSocket-клиент агента.
type Client struct {
	cfg     *Config
	handler *CommandHandler
	pendingSystemLogs []shared.AgentSystemLogEvent

	ws      *websocket.Conn
	send    chan []byte
	mu      sync.Mutex
	writeMu sync.Mutex
	closed  bool
}

// NewClient создаёт клиент.
func NewClient(cfg *Config) *Client {
	client := &Client{
		cfg:     cfg,
		handler: NewCommandHandler(cfg),
		send:    make(chan []byte, 32),
	}
	client.handler.processes.SetLogEmitter(func(siteID, line string) {
		if err := client.SendEvent("log", shared.LogEvent{SiteId: siteID, Line: line}); err != nil && err.Error() != "not connected" {
			log.Printf("stream site log: %v", err)
		}
	})
	client.handler.processes.SetStatusEmitter(func(siteID, status string, pid int) {
		if err := client.SendEvent("status", shared.StatusEvent{SiteId: siteID, Status: status, Pid: uint32(max(pid, 0))}); err != nil && err.Error() != "not connected" {
			log.Printf("stream site status: %v", err)
		}
	})
	client.handler.processes.SetDeploymentEmitter(func(event shared.DeploymentJobEvent) {
		if err := client.SendEvent("deployment_job", event); err != nil && err.Error() != "not connected" {
			log.Printf("stream deployment job: %v", err)
		}
	})
	return client
}


var agentSensitiveLogPattern = regexp.MustCompile(`(?i)(token|secret|password|authorization|private[_ -]?key|client_secret|access_token|refresh_token|code)(["']?\s*[:=]\s*["']?)[^"'\s&,;]+`)
var agentBearerPattern = regexp.MustCompile(`(?i)bearer\s+[a-z0-9._~+/-]+=*`)

type agentLogForwarder struct { client *Client }

func (w agentLogForwarder) Write(p []byte) (int, error) {
	line := strings.TrimSpace(string(p))
	if line == "" { return len(p), nil }
	level := "info"
	lower := strings.ToLower(line)
	if strings.Contains(lower, "warning") || strings.Contains(lower, "warn:") { level = "warn" }
	if strings.Contains(lower, "error") || strings.Contains(lower, "failed") || strings.Contains(lower, "failure") { level = "error" }
	if level != "info" {
		line = agentSensitiveLogPattern.ReplaceAllString(line, "$1$2[REDACTED]")
		line = agentBearerPattern.ReplaceAllString(line, "Bearer [REDACTED]")
		if len(line) > 512 { line = line[:512] }
		w.client.emitSystemLog(level, "agent-runtime", "Agent runtime: "+line, nil)
	}
	return len(p), nil
}

// ForwardLogs mirrors Agent warnings/errors to the Control Plane while preserving
// the existing stderr and private-file log destinations.
func (c *Client) ForwardLogs(previous io.Writer) {
	if previous == nil { previous = io.Discard }
	log.SetOutput(io.MultiWriter(previous, agentLogForwarder{client:c}))
}

// Run — главный цикл. Подключается, переподключается при обрыве.
func (c *Client) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := c.connect(ctx); err != nil {
			log.Printf("connection failed: %v (retry in %s)", err, ReconnectDelay)
			c.queueSystemLog("error", "connection", "Agent could not connect to Control Plane", map[string]interface{}{"cause": err.Error(), "retry_after_seconds": int(ReconnectDelay.Seconds())})
			time.Sleep(ReconnectDelay)
			continue
		}

		c.runSession(ctx)

		c.mu.Lock()
		closed := c.closed
		c.mu.Unlock()
		if closed {
			return nil
		}

		log.Printf("connection lost, reconnecting in %s...", ReconnectDelay)
		c.queueSystemLog("warn", "connection", "Agent connection was lost", map[string]interface{}{"retry_after_seconds": int(ReconnectDelay.Seconds())})
		time.Sleep(ReconnectDelay)
	}
}

// Stop останавливает клиент.
func (c *Client) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	if c.ws != nil {
		_ = c.ws.Close()
	}
}

// ============================================================
// CONNECT
// ============================================================

func (c *Client) connect(ctx context.Context) error {
	if c.cfg.ServerURL == "" {
		return errors.New("server_url is empty (edit agent.json)")
	}
	if c.cfg.Token == "" {
		return errors.New("token is empty (edit agent.json)")
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: false},
	}

	log.Printf("connecting to %s...", c.cfg.ServerURL)
	ws, _, err := dialer.DialContext(ctx, c.cfg.ServerURL, http.Header{})
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	c.mu.Lock()
	c.ws = ws
	c.mu.Unlock()

	// A failed handshake must not leave a socket behind while Run retries.
	authenticated := false
	defer func() {
		if authenticated {
			return
		}
		_ = ws.Close()
		c.mu.Lock()
		if c.ws == ws {
			c.ws = nil
		}
		c.mu.Unlock()
	}()

	ws.SetReadLimit(MaxMessageSize)
	_ = ws.SetReadDeadline(time.Now().Add(PongWait))
	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(time.Now().Add(PongWait))
	})

	tailscaleIP, err := GetTailscaleIP()
	if err != nil {
		log.Printf("warning: tailscale: %v", err)
		c.queueSystemLog("warn", "agent", "Tailscale address detection failed", map[string]interface{}{"cause": err.Error()})
	}

	// Отправляем AuthRequest.
	authEnv := shared.Envelope{
		AuthReq: &shared.AuthRequest{
			Token:        c.cfg.Token,
			DeviceId:     c.cfg.DeviceID,
			Hostname:     c.cfg.resolveHostname(),
			TailscaleIp:  tailscaleIP,
			AgentVersion: Version,
			Platform: runtime.GOOS,
			Architecture: runtime.GOARCH,
			Capabilities: detectAgentCapabilities(),
		},
	}

	msg, err := shared.Marshal(authEnv)
	if err != nil {
		return fmt.Errorf("marshal auth: %w", err)
	}

	c.writeMu.Lock()
	_ = ws.SetWriteDeadline(time.Now().Add(WriteWait))
	err = ws.WriteMessage(websocket.BinaryMessage, msg)
	c.writeMu.Unlock()
	if err != nil {
		return fmt.Errorf("write auth: %w", err)
	}

	// Ждём AuthResponse.
	_ = ws.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, respBytes, err := ws.ReadMessage()
	if err != nil {
		return fmt.Errorf("read auth response: %w", err)
	}

	var respEnv shared.Envelope
	if err := shared.Unmarshal(respBytes, &respEnv); err != nil {
		return fmt.Errorf("unmarshal auth response: %w", err)
	}

	if respEnv.AuthResp == nil {
		return errors.New("expected auth response")
	}

	if !respEnv.AuthResp.Ok {
		return fmt.Errorf("auth rejected: %s", respEnv.AuthResp.Message)
	}

	// The short deadline above is only for the authentication handshake.
	// Restore the heartbeat deadline before the long-lived session starts.
	_ = ws.SetReadDeadline(time.Now().Add(PongWait))
	authenticated = true
	log.Printf("authenticated as user %d", respEnv.AuthResp.UserId)
	c.flushSystemLogs()
	c.emitSystemLog("info", "connection", "Agent connected and authenticated", map[string]interface{}{"agent_version": Version, "platform": runtime.GOOS, "architecture": runtime.GOARCH})
	return nil
}

// ============================================================
// SESSION
// ============================================================

func (c *Client) runSession(ctx context.Context) {
	done := make(chan struct{})
	var once sync.Once
	stop := func() {
		once.Do(func() { close(done) })
	}

	go func() {
		defer stop()
		c.writePump()
	}()

	go func() {
		defer stop()
		c.readPump(ctx)
	}()

	go c.metricsPump(ctx, done)

	<-done

	c.mu.Lock()
	if c.ws != nil {
		_ = c.ws.Close()
		c.ws = nil
	}
	c.mu.Unlock()
}

// readPump читает сообщения от сервера.
func (c *Client) readPump(ctx context.Context) {
	defer func() {
		c.mu.Lock()
		if c.ws != nil {
			_ = c.ws.Close()
		}
		c.mu.Unlock()
	}()

	for {
		c.mu.Lock()
		ws := c.ws
		c.mu.Unlock()
		if ws == nil {
			return
		}

		_, msg, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("read: %v", err)
			}
			return
		}

		var env shared.Envelope
		if err := shared.Unmarshal(msg, &env); err != nil {
			log.Printf("unmarshal: %v", err)
			continue
		}

		if env.Command != nil {
			go c.handleCommand(ctx, env.Command)
		}
	}
}

// writePump пишет сообщения серверу.
func (c *Client) writePump() {
	ticker := time.NewTicker(PingPeriod)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		ws := c.ws
		c.mu.Unlock()
		if ws == nil {
			return
		}

		c.writeMu.Lock()
		_ = ws.SetWriteDeadline(time.Now().Add(WriteWait))
		pingPayload, _ := shared.Marshal(shared.Envelope{Event: &shared.Event{Event: "ping"}})
		if err := ws.WriteMessage(websocket.BinaryMessage, pingPayload); err != nil {
			c.writeMu.Unlock()
			return
		}
		if err := ws.WriteMessage(websocket.PingMessage, nil); err != nil {
			c.writeMu.Unlock()
			return
		}
		c.writeMu.Unlock()
	}
}

// ============================================================
// ОБРАБОТКА КОМАНД
// ============================================================

func (c *Client) handleCommand(ctx context.Context, cmd *shared.Command) {
	result := c.handler.HandleCommand(cmd)

	respEnv := shared.Envelope{
		Result: result,
	}

	msg, err := shared.Marshal(respEnv)
	if err != nil {
		log.Printf("marshal result: %v", err)
		return
	}

	c.mu.Lock()
	ws := c.ws
	c.mu.Unlock()
	if ws == nil {
		return
	}

	_ = ws.SetWriteDeadline(time.Now().Add(WriteWait))
	c.writeMu.Lock()
	_ = ws.SetWriteDeadline(time.Now().Add(WriteWait))
	err = ws.WriteMessage(websocket.BinaryMessage, msg)
	c.writeMu.Unlock()
	if err != nil {
		log.Printf("write result: %v", err)
	}

	ok := result != nil && result.Ok
	log.Printf("command %s done: ok=%v", cmd.Id, ok)
	level := "info"
	if !ok { level = "error" }
	details := map[string]interface{}{"command_id": cmd.Id, "action": cmd.Action}
	if cmd.TraceID != "" { details["request_id"] = cmd.TraceID }
	if result != nil && !result.Ok && result.Message != "" { details["cause"] = result.Message }
	message := "Agent command completed"
	if level == "error" { message = "Agent command failed" }
	category := "command"
	switch {
	case strings.HasPrefix(cmd.Action, "build"), strings.Contains(cmd.Action, "build_history"), strings.Contains(cmd.Action, "build_artifact"): category = "builds"
	case strings.Contains(cmd.Action, "deploy"), strings.Contains(cmd.Action, "rollback"), strings.Contains(cmd.Action, "release"): category = "deployments"
	case strings.Contains(cmd.Action, "database"): category = "database"
	case strings.Contains(cmd.Action, "backup"): category = "backups"
	}
	c.emitSystemLog(level, category, message, details)
}

// ============================================================
// ОТПРАВКА СОБЫТИЙ
// ============================================================

// SendEvent отправляет событие серверу (например, лог или ping).
func (c *Client) SendEvent(event string, payload interface{}) error {
	payloadBytes, err := shared.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}

	env := shared.Envelope{
		Event: &shared.Event{
			Event:   event,
			Payload: payloadBytes,
		},
	}

	msg, err := shared.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	c.mu.Lock()
	ws := c.ws
	c.mu.Unlock()
	if ws == nil {
		return errors.New("not connected")
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = ws.SetWriteDeadline(time.Now().Add(WriteWait))
	return ws.WriteMessage(websocket.BinaryMessage, msg)
}



func (c *Client) queueSystemLog(level, category, message string, metadata map[string]interface{}) {
	entry := shared.AgentSystemLogEvent{Level: level, Category: category, Source: "agent", Message: message, Timestamp: time.Now().UTC(), Metadata: metadata}
	c.mu.Lock()
	if len(c.pendingSystemLogs) >= 100 { c.pendingSystemLogs = c.pendingSystemLogs[1:] }
	c.pendingSystemLogs = append(c.pendingSystemLogs, entry)
	c.mu.Unlock()
}

func (c *Client) emitSystemLog(level, category, message string, metadata map[string]interface{}) {
	entry := shared.AgentSystemLogEvent{Level: level, Category: category, Source: "agent", Message: message, Timestamp: time.Now().UTC(), Metadata: metadata}
	if err := c.SendEvent("system_log", entry); err != nil { c.queueSystemLog(level, category, message, metadata) }
}

func (c *Client) flushSystemLogs() {
	c.mu.Lock()
	pending := append([]shared.AgentSystemLogEvent(nil), c.pendingSystemLogs...)
	c.pendingSystemLogs = nil
	c.mu.Unlock()
	for i, entry := range pending {
		if err := c.SendEvent("system_log", entry); err != nil {
			c.mu.Lock()
			c.pendingSystemLogs = append(pending[i:], c.pendingSystemLogs...)
			if len(c.pendingSystemLogs) > 100 { c.pendingSystemLogs = c.pendingSystemLogs[len(c.pendingSystemLogs)-100:] }
			c.mu.Unlock()
			return
		}
	}
}

// metricsPump periodically pushes device metrics to the Control Plane.
func (c *Client) metricsPump(ctx context.Context, done <-chan struct{}) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			metrics, err := collectMetrics(c.cfg.DataRoot)
			if err != nil {
				log.Printf("metrics collection: %v", err)
				c.queueSystemLog("warn", "monitoring", "Agent metrics collection failed", map[string]interface{}{"cause": err.Error()})
				continue
			}
			if err := c.SendEvent("metrics", metrics); err != nil {
				log.Printf("metrics push: %v", err)
				c.queueSystemLog("warn", "monitoring", "Agent metrics could not be sent", map[string]interface{}{"cause": err.Error()})
				return
			}
		}
	}
}
