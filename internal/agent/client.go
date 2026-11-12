// Package agent — часть VELTRIX Dock, которая работает на телефонах пользователей.
// Этот файл: WebSocket-клиент (подключение к серверу, приём команд, отправка ответов).
package agent

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"veltrix/internal/shared"
)

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

	ws      *websocket.Conn
	send    chan []byte
	mu      sync.Mutex
	writeMu sync.Mutex
	closed  bool
}

// NewClient создаёт клиент.
func NewClient(cfg *Config) *Client {
	return &Client{
		cfg:     cfg,
		handler: NewCommandHandler(cfg),
		send:    make(chan []byte, 32),
	}
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

	ws.SetReadLimit(MaxMessageSize)
	_ = ws.SetReadDeadline(time.Now().Add(PongWait))
	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(time.Now().Add(PongWait))
	})

	tailscaleIP, err := GetTailscaleIP()
	if err != nil {
		log.Printf("warning: tailscale: %v", err)
	}

	// Отправляем AuthRequest.
	authEnv := shared.Envelope{
		AuthReq: &shared.AuthRequest{
			Token:        c.cfg.Token,
			DeviceId:     c.cfg.DeviceID,
			Hostname:     c.cfg.resolveHostname(),
			TailscaleIp:  tailscaleIP,
			AgentVersion: Version,
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

	log.Printf("authenticated as user %d", respEnv.AuthResp.UserId)
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
	if err := ws.WriteMessage(websocket.BinaryMessage, msg); err != nil {
		log.Printf("write result: %v", err)
	}

	log.Printf("command %s done: ok=%v", cmd.Id, result.Ok)
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
