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
	"google.golang.org/protobuf/proto"
	pb "veltrix/internal/shared"
)

// Константы.
const (
	Version          = "1.0.0"
	ReconnectDelay   = 5 * time.Second  // пауза между переподключениями
	WriteWait        = 10 * time.Second // таймаут записи
	PongWait         = 60 * time.Second // ждём pong от сервера
	PingPeriod       = 30 * time.Second // шлём ping каждые 30 сек
	MaxMessageSize   = 1 << 20          // 1 MB
)

// Client — WebSocket-клиент агента.
type Client struct {
	cfg     *Config
	handler *CommandHandler

	ws      *websocket.Conn
	send    chan []byte
	mu      sync.Mutex
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

// Run — главный цикл. Подключается к серверу, переподключается при обрыве.
// Блокируется, пока не будет вызван Stop().
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

		// Подключились, работаем.
		c.runSession(ctx)

		// Сессия закончилась (обрыв или Stop).
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

// connect подключается к серверу и проходит аутентификацию.
func (c *Client) connect(ctx context.Context) error {
	if c.cfg.ServerURL == "" {
		return errors.New("server_url is empty (edit agent.json)")
	}
	if c.cfg.Token == "" {
		return errors.New("token is empty (edit agent.json)")
	}

	// Настраиваем dialer (для ws:// без TLS — обычный, для wss:// — с TLS).
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

	// Настраиваем лимиты.
	ws.SetReadLimit(MaxMessageSize)
	_ = ws.SetReadDeadline(time.Now().Add(PongWait))
	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(time.Now().Add(PongWait))
	})

	// Получаем IP из Tailscale (если не удалось — пустая строка).
	tailscaleIP, err := GetTailscaleIP()
	if err != nil {
		log.Printf("warning: tailscale: %v", err)
	}

	// Отправляем AuthRequest.
	authReq := &pb.Envelope{
		Payload: &pb.Envelope_AuthReq{
			AuthReq: &pb.AuthRequest{
				Token:        c.cfg.Token,
				DeviceId:     c.cfg.DeviceID,
				Hostname:     c.cfg.resolveHostname(),
				TailscaleIp:  tailscaleIP,
				AgentVersion: Version,
			},
		},
	}

	msg, err := proto.Marshal(authReq)
	if err != nil {
		return fmt.Errorf("marshal auth: %w", err)
	}

	_ = ws.SetWriteDeadline(time.Now().Add(WriteWait))
	if err := ws.WriteMessage(websocket.BinaryMessage, msg); err != nil {
		return fmt.Errorf("write auth: %w", err)
	}

	// Ждём AuthResponse.
	_ = ws.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, respBytes, err := ws.ReadMessage()
	if err != nil {
		return fmt.Errorf("read auth response: %w", err)
	}

	var respEnv pb.Envelope
	if err := proto.Unmarshal(respBytes, &respEnv); err != nil {
		return fmt.Errorf("unmarshal auth response: %w", err)
	}

	authResp, ok := respEnv.Payload.(*pb.Envelope_AuthResp)
	if !ok {
		return errors.New("expected auth response")
	}

	if !authResp.AuthResp.Ok {
		return fmt.Errorf("auth rejected: %s", authResp.AuthResp.Message)
	}

	log.Printf("authenticated as user %d", authResp.AuthResp.UserId)
	return nil
}

// ============================================================
// SESSION
// ============================================================

// runSession запускает reader и writer, ждёт закрытия.
func (c *Client) runSession(ctx context.Context) {
	done := make(chan struct{})
	var once sync.Once
	stop := func() {
		once.Do(func() { close(done) })
	}

	// Writer.
	go func() {
		defer stop()
		c.writePump()
	}()

	// Reader.
	go func() {
		defer stop()
		c.readPump(ctx)
	}()

	// Ждём завершения любой из горутин.
	<-done

	// Закрываем соединение.
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

		// Парсим Envelope.
		var env pb.Envelope
		if err := proto.Unmarshal(msg, &env); err != nil {
			log.Printf("unmarshal: %v", err)
			continue
		}

		// Обрабатываем Command.
		if cmd, ok := env.Payload.(*pb.Envelope_Command); ok {
			go c.handleCommand(ctx, cmd.Command)
		}
	}
}

// writePump пишет сообщения серверу (единственная горутина-писатель).
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

		// Отправляем ping (WebSocket-уровень).
		_ = ws.SetWriteDeadline(time.Now().Add(WriteWait))
		if err := ws.WriteMessage(websocket.PingMessage, nil); err != nil {
			return
		}
	}
}

// ============================================================
// ОБРАБОТКА КОМАНД
// ============================================================

// handleCommand обрабатывает одну команду и отправляет Result.
func (c *Client) handleCommand(ctx context.Context, cmd *pb.Command) {
	// Обрабатываем (это может занять время).
	result := c.handler.HandleCommand(cmd)

	// Отправляем Result серверу.
	respEnv := &pb.Envelope{
		Payload: &pb.Envelope_Result{
			Result: result,
		},
	}

	msg, err := proto.Marshal(respEnv)
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
func (c *Client) SendEvent(event string, payload proto.Message) error {
	payloadBytes, err := proto.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}

	env := &pb.Envelope{
		Payload: &pb.Envelope_Event{
			Event: &pb.Event{
				Event:   event,
				Payload: payloadBytes,
			},
		},
	}

	msg, err := proto.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	c.mu.Lock()
	ws := c.ws
	c.mu.Unlock()
	if ws == nil {
		return errors.New("not connected")
	}

	_ = ws.SetWriteDeadline(time.Now().Add(WriteWait))
	return ws.WriteMessage(websocket.BinaryMessage, msg)
}
