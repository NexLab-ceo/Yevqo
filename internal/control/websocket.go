// Package control — серверная часть VELTRIX Dock (Redmi 9).
// Этот файл: WebSocket-хаб для связи с агентами.
package control

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	"veltrix/internal/shared"
	pb "veltrix/internal/shared"
)

// Константы WebSocket.
const (
	wsWriteWait      = 10 * time.Second // таймаут записи
	wsPongWait       = 60 * time.Second // ждём pong от агента
	wsPingPeriod     = 30 * time.Second // шлём ping каждые 30 сек
	wsMaxMessageSize = 1 << 20          // 1 MB — максимум на сообщение
	wsCommandTimeout = 30 * time.Second // сколько ждём Result на команду
)

// upgrader — превращает HTTP-запрос в WebSocket.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Агенты ходят с телефонов, у них нет браузерного Origin.
		// Пропускаем всех — защита в AuthRequest.
		return true
	},
}

// ============================================================
// HUB — реестр всех подключённых агентов
// ============================================================

// Hub держит все активные соединения.
type Hub struct {
	mu    sync.RWMutex
	conns map[int64]*AgentConn // key = device.ID (внутренний)
}

// NewHub создаёт пустой хаб.
func NewHub() *Hub {
	return &Hub{conns: make(map[int64]*AgentConn)}
}

// Add регистрирует соединение.
func (h *Hub) Add(a *AgentConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[a.Device.ID] = a
}

// Remove удаляет соединение.
func (h *Hub) Remove(deviceID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.conns, deviceID)
}

// Get возвращает соединение по deviceID.
func (h *Hub) Get(deviceID int64) (*AgentConn, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	c, ok := h.conns[deviceID]
	return c, ok
}

// List возвращает копию списка всех соединений.
func (h *Hub) List() []*AgentConn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]*AgentConn, 0, len(h.conns))
	for _, c := range h.conns {
		out = append(out, c)
	}
	return out
}

// ============================================================
// AGENTCONN — одно соединение с агентом
// ============================================================

// AgentConn — это один подключённый телефон.
type AgentConn struct {
	Device *shared.Device
	ws     *websocket.Conn

	// send — канал исходящих сообщений (Protobuf-байты).
	// Один writer-goroutine читает из него и пишет в ws.
	send chan []byte

	// pending — ожидающие ответа команды (key = command.ID).
	pending   map[string]chan *pb.Result
	pendingMu sync.Mutex

	hub *Hub
	db  *DB
}

// ============================================================
// ОБРАБОТЧИК: /ws (приём агентов)
// ============================================================

// handleWebSocket — HTTP-хендлер для /ws. Вызывается из server.go.
func (srv *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Апгрейд до WebSocket.
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade: %v", err)
		return
	}

	// Создаём соединение (пока без Device — заполнится после AuthRequest).
	conn := &AgentConn{
		ws:      ws,
		send:    make(chan []byte, 32),
		pending: make(map[string]chan *pb.Result),
		hub:     srv.hub,
		db:      srv.db,
	}

	// Запускаем writer и reader.
	go conn.writePump()
	conn.readPump(srv)
}

// readPump — читает сообщения от агента.
func (c *AgentConn) readPump(srv *Server) {
	defer func() {
		c.hub.Remove(c.Device.ID)
		_ = c.db.MarkDeviceOffline(context.Background(), c.Device.ID)
		c.ws.Close()
	}()

	c.ws.SetReadLimit(wsMaxMessageSize)
	_ = c.ws.SetReadDeadline(time.Now().Add(wsPongWait))
	c.ws.SetPongHandler(func(string) error {
		_ = c.ws.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	for {
		_, msg, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("ws read: %v", err)
			}
			return
		}

		// Парсим Envelope.
		var env pb.Envelope
		if err := proto.Unmarshal(msg, &env); err != nil {
			log.Printf("ws unmarshal: %v", err)
			continue
		}

		// Разбираем тип сообщения.
		switch p := env.Payload.(type) {
		case *pb.Envelope_AuthReq:
			c.handleAuth(srv, p.AuthReq)
		case *pb.Envelope_Result:
			c.handleResult(p.Result)
		case *pb.Envelope_Event:
			c.handleEvent(srv, p.Event)
		default:
			log.Printf("ws unknown message type")
		}
	}
}

// writePump — пишет сообщения агенту (единственная горутина, которая пишет).
func (c *AgentConn) writePump() {
	ticker := time.NewTicker(wsPingPeriod)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.ws.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if !ok {
				_ = c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.ws.WriteMessage(websocket.BinaryMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.ws.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ============================================================
// ОБРАБОТЧИКИ СООБЩЕНИЙ
// ============================================================

// handleAuth — аутентификация агента при подключении.
func (c *AgentConn) handleAuth(srv *Server, req *pb.AuthRequest) {
	ctx := context.Background()

	// Ищем пользователя по токену привязки.
	// В нашей схеме token = referral_code пользователя (см. auth.go).
	user, err := srv.db.GetUserByReferralCode(ctx, req.Token)
	if err != nil {
		c.sendAuthError("invalid token")
		return
	}

	if user.Status == shared.UserBlocked {
		c.sendAuthError("user is blocked")
		return
	}

	// Проверяем лимит устройств.
	count, _ := srv.db.CountDevicesByUser(ctx, user.ID)
	if count >= user.MaxDevicesTotal() {
		// Устройства с этим UUID ещё нет? Тогда лимит превышен.
		existing, _ := srv.db.GetDeviceByUUID(ctx, req.DeviceId)
		if existing == nil {
			c.sendAuthError("device limit reached")
			return
		}
	}

	// Upsert устройства.
	device := &shared.Device{
		UserID:       user.ID,
		DeviceID:     req.DeviceId,
		Hostname:     req.Hostname,
		TailscaleIP:  req.TailscaleIp,
		AgentVersion: req.AgentVersion,
	}
	device, err = srv.db.UpsertDevice(ctx, device)
	if err != nil {
		c.sendAuthError("db error")
		log.Printf("upsert device: %v", err)
		return
	}

	c.Device = device
	c.hub.Add(c)

	// Отвечаем.
	resp := &pb.Envelope{
		Payload: &pb.Envelope_AuthResp{
			AuthResp: &pb.AuthResponse{
				Ok:     true,
				Message: "welcome",
				UserId: user.ID,
			},
		},
	}
	b, _ := proto.Marshal(resp)
	c.send <- b
}

// sendAuthError — отправляет AuthResponse с ошибкой.
func (c *AgentConn) sendAuthError(msg string) {
	resp := &pb.Envelope{
		Payload: &pb.Envelope_AuthResp{
			AuthResp: &pb.AuthResponse{Ok: false, Message: msg},
		},
	}
	b, _ := proto.Marshal(resp)
	c.send <- b
}

// handleResult — ответ агента на команду.
func (c *AgentConn) handleResult(res *pb.Result) {
	c.pendingMu.Lock()
	ch, ok := c.pending[res.Id]
	if ok {
		delete(c.pending, res.Id)
	}
	c.pendingMu.Unlock()

	if ok {
		ch <- res // отправляем в ожидающий SendCommand
	}
}

// handleEvent — события от агента (логи, статусы, ping).
func (c *AgentConn) handleEvent(srv *Server, ev *pb.Event) {
	switch ev.Event {
	case "ping":
		// Агент жив — можно обновить last_seen в БД (опционально).
		_ = srv.db.UpdateUserLastSeen(context.Background(), c.Device.UserID)

	case "log":
		var le pb.LogEvent
		if err := proto.Unmarshal(ev.Payload, &le); err == nil {
			// Сохраняем или пробрасываем на фронт (пока — просто лог).
			log.Printf("[%s/%s] %s", c.Device.Hostname, le.SiteId, le.Line)
		}

	case "status":
		var se pb.StatusEvent
		if err := proto.Unmarshal(ev.Payload, &se); err == nil {
			_ = srv.db.UpdateSiteStatus(
				context.Background(), c.Device.ID, se.SiteId, se.Status, int(se.Pid),
			)
		}
	}
}

// ============================================================
// ОТПРАВКА КОМАНД АГЕНТУ
// ============================================================

// SendCommand отправляет команду агенту и ждёт Result (с таймаутом).
func (c *AgentConn) SendCommand(ctx context.Context, action string, payload proto.Message) (*pb.Result, error) {
	// Генерируем ID команды.
	cmdID, err := GenerateToken(16)
	if err != nil {
		return nil, err
	}

	// Сериализуем payload.
	var payloadBytes []byte
	if payload != nil {
		payloadBytes, err = proto.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}
	}

	// Формируем Command.
	cmd := &pb.Envelope{
		Payload: &pb.Envelope_Command{
			Command: &pb.Command{
				Id:      cmdID,
				Action:  action,
				Payload: payloadBytes,
			},
		},
	}
	msg, err := proto.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("marshal command: %w", err)
	}

	// Создаём канал для ответа.
	ch := make(chan *pb.Result, 1)
	c.pendingMu.Lock()
	c.pending[cmdID] = ch
	c.pendingMu.Unlock()

	// Отправляем в очередь writer'а.
	select {
	case c.send <- msg:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(wsWriteWait):
		return nil, fmt.Errorf("write timeout")
	}

	// Ждём Result с таймаутом.
	select {
	case res := <-ch:
		return res, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(wsCommandTimeout):
		c.pendingMu.Lock()
		delete(c.pending, cmdID)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("command timeout: %s", action)
	}
}

// ============================================================
// ХЕНДЛЕР ДЛЯ API: получить список онлайн-устройств
// ============================================================

// OnlineDevices — возвращает список подключённых устройств (для API).
func (srv *Server) OnlineDevices() []map[string]interface{} {
	conns := srv.hub.List()
	out := make([]map[string]interface{}, 0, len(conns))
	for _, c := range conns {
		out = append(out, map[string]interface{}{
			"id":         c.Device.ID,
			"device_id":  c.Device.DeviceID,
			"hostname":   c.Device.Hostname,
			"tailscale":  c.Device.TailscaleIP,
			"user_id":    c.Device.UserID,
		})
	}
	return out
}

// helper для JSON-логов (не используется в протоколе, но удобно).
func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
