// Package control — серверная часть VELTRIX Dock (Redmi 9).
// Этот файл: WebSocket-хаб для связи с агентами.
package control

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"veltrix/internal/shared"
)

const (
	wsWriteWait      = 10 * time.Second
	wsPongWait       = 60 * time.Second
	wsPingPeriod     = 30 * time.Second
	wsMaxMessageSize = 1 << 20
	wsCommandTimeout = 10 * time.Minute
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" { return true } // native Termux Agent has no browser Origin
		u, err := url.Parse(origin)
		return err == nil && (u.Scheme == "http" || u.Scheme == "https") && strings.EqualFold(u.Host, r.Host)
	},
}

// Hub — реестр подключённых агентов.
type Hub struct {
	mu    sync.RWMutex
	conns map[int64]*AgentConn
}

func NewHub() *Hub {
	return &Hub{conns: make(map[int64]*AgentConn)}
}

func (h *Hub) Add(a *AgentConn) {
	h.mu.Lock()
	previous := h.conns[a.Device.ID]
	h.conns[a.Device.ID] = a
	h.mu.Unlock()
	// A device has one active socket. Close the replaced connection after
	// releasing the hub lock; its deferred cleanup cannot remove the new one.
	if previous != nil && previous != a && previous.ws != nil {
		_ = previous.ws.Close()
	}
}

func (h *Hub) Remove(deviceID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.conns, deviceID)
}

// RemoveConn prevents an old socket's teardown from removing a newer Agent session.
func (h *Hub) RemoveConn(a *AgentConn) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conns[a.Device.ID] != a { return false }
	delete(h.conns, a.Device.ID)
	return true
}

func (h *Hub) Get(deviceID int64) (*AgentConn, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	c, ok := h.conns[deviceID]
	return c, ok
}

func (h *Hub) List() []*AgentConn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]*AgentConn, 0, len(h.conns))
	for _, c := range h.conns {
		out = append(out, c)
	}
	return out
}

// AgentConn — одно соединение с агентом.
type AgentConn struct {
	Device    *shared.Device
	ws        *websocket.Conn
	send      chan []byte
	pending   map[string]chan *shared.Result
	pendingMu sync.Mutex
	done      chan struct{}
	doneOnce  sync.Once
	hub       *Hub
	db        *DB
}

func (srv *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade: %v", err)
		return
	}

	conn := &AgentConn{
		ws:      ws,
		send:    make(chan []byte, 32),
		pending: make(map[string]chan *shared.Result),
		done:    make(chan struct{}),
		hub:     srv.hub,
		db:      srv.db,
	}

	go conn.writePump()
	conn.readPump(srv)
}

func (c *AgentConn) readPump(srv *Server) {
	defer func() {
		c.shutdown()
		if c.Device != nil {
			if c.hub.RemoveConn(c) {
				offlineCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				if err := c.db.MarkDeviceOffline(offlineCtx, c.Device.ID); err == nil {
					if alertErr := srv.evaluateDeviceConnectivityAlert(offlineCtx, c.Device.UserID, c.Device.ID, false); alertErr != nil { log.Printf("evaluate device offline alert: %v", alertErr) }
				}
				cancel()
				if srv.logStreams != nil { srv.logStreams.publishAgentStatus(c.Device.UserID, c.Device.ID, "offline") }
			}
		}
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

		var env shared.Envelope
		if err := shared.Unmarshal(msg, &env); err != nil {
			log.Printf("ws unmarshal: %v", err)
			continue
		}

		if c.Device == nil && env.AuthReq == nil { continue }
		switch {
		case env.AuthReq != nil:
			c.handleAuth(srv, env.AuthReq)
		case env.Result != nil:
			c.handleResult(env.Result)
		case env.Event != nil:
			c.handleEvent(srv, env.Event)
		}
	}
}

func (c *AgentConn) writePump() {
	ticker := time.NewTicker(wsPingPeriod)
	defer func() {
		ticker.Stop()
		c.shutdown()
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

func (c *AgentConn) handleAuth(srv *Server, req *shared.AuthRequest) {
	ctx := context.Background()

	if c.Device != nil {
		c.sendAuthError("connection is already authenticated")
		return
	}

	user, err := srv.db.UserByAgentToken(ctx, req.Token)
	if err != nil {
		c.sendAuthError("invalid token")
		return
	}

	if user.Status == shared.UserBlocked {
		c.sendAuthError("user is blocked")
		return
	}

	device := &shared.Device{
		UserID:       user.ID,
		DeviceID:     req.DeviceId,
		Hostname:     req.Hostname,
		TailscaleIP:  req.TailscaleIp,
		AgentVersion: req.AgentVersion,
		Platform: req.Platform,
		Architecture: req.Architecture,
		Capabilities: req.Capabilities,
	}
	device, err = srv.db.UpsertDevice(ctx, device)
	if err != nil {
		var apiErr *shared.APIError
		if errors.As(err, &apiErr) {
			c.sendAuthError(apiErr.Message)
		} else {
			c.sendAuthError("device registration failed")
		}
		log.Printf("upsert device: %v", err)
		return
	}

	c.Device = device
	c.hub.Add(c)
	if alertErr := srv.evaluateDeviceConnectivityAlert(ctx, device.UserID, device.ID, true); alertErr != nil { log.Printf("evaluate device online alert: %v", alertErr) }
	if srv.logStreams != nil { srv.logStreams.publishAgentStatus(device.UserID, device.ID, "online") }

	resp := shared.Envelope{
		AuthResp: &shared.AuthResponse{
			Ok:      true,
			Message: "welcome",
			UserId:  user.ID,
		},
	}
	b, _ := shared.Marshal(resp)
	c.send <- b
}

func (c *AgentConn) sendAuthError(msg string) {
	resp := shared.Envelope{
		AuthResp: &shared.AuthResponse{Ok: false, Message: msg},
	}
	b, _ := shared.Marshal(resp)
	c.send <- b
}

func (c *AgentConn) shutdown() {
	c.doneOnce.Do(func() {
		if c.done != nil { close(c.done) }
		c.pendingMu.Lock()
		pending := c.pending
		c.pending = make(map[string]chan *shared.Result)
		c.pendingMu.Unlock()
		for _, ch := range pending {
			select { case ch <- &shared.Result{Ok:false,Message:"Agent disconnected"}: default: }
		}
	})
}

func (c *AgentConn) handleResult(res *shared.Result) {
	c.pendingMu.Lock()
	ch, ok := c.pending[res.Id]
	if ok {
		delete(c.pending, res.Id)
	}
	c.pendingMu.Unlock()
	if ok {
		ch <- res
	}
}

func (c *AgentConn) handleEvent(srv *Server, ev *shared.Event) {
	switch ev.Event {
	case "ping":
		if c.Device != nil {
			_ = srv.db.UpdateUserLastSeen(context.Background(), c.Device.UserID)
			_ = srv.db.UpdateDeviceHeartbeat(context.Background(), c.Device.ID)
		}
	case "log":
		var le shared.LogEvent
		if err := shared.Unmarshal(ev.Payload, &le); err == nil && c.Device != nil {
			log.Printf("[%s/%s] %s", c.Device.Hostname, le.SiteId, le.Line)
			if srv.logStreams != nil {
				srv.logStreams.publish(c.Device.UserID, c.Device.ID, le)
			}
		}
	case "metrics":
		var metrics shared.MetricsResult
		if err := shared.Unmarshal(ev.Payload, &metrics); err == nil && c.Device != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			_, dbErr := srv.db.conn.ExecContext(ctx,
				"INSERT INTO device_metrics(device_id,cpu_percent,load_1m,memory_total_bytes,memory_available_bytes,storage_total_bytes,storage_available_bytes,temperature_celsius,captured_at) VALUES(?,?,?,?,?,?,?,?,?)",
				c.Device.ID, metrics.CPUPercent, metrics.Load1m, metrics.MemoryTotalBytes, metrics.MemoryAvailableBytes,
				metrics.StorageTotalBytes, metrics.StorageAvailableBytes, metrics.TemperatureCelsius, metrics.CapturedAt)
			if dbErr == nil {
				if alertErr := srv.evaluateDeviceMetricAlerts(ctx, c.Device.UserID, c.Device.ID, metrics); alertErr != nil { log.Printf("evaluate device metric alerts: %v", alertErr) }
			}
			cancel()
			if dbErr != nil { log.Printf("store pushed metrics: %v", dbErr) }
		}
	case "deployment_job":
		var job shared.DeploymentJobEvent
		if err := shared.Unmarshal(ev.Payload, &job); err == nil && c.Device != nil && validLogSiteID(job.SiteId) {
			if srv.logStreams != nil { srv.logStreams.publishDeployment(c.Device.UserID, c.Device.ID, job) }
		}
	case "status":
		var se shared.StatusEvent
		if err := shared.Unmarshal(ev.Payload, &se); err == nil && c.Device != nil && validLogSiteID(se.SiteId) {
			// Agent status events are asynchronous and may arrive while a connection
			// is being torn down. Never dereference an unauthenticated/disconnected
			// device context, and reject malformed site identifiers before DB writes.
			_ = srv.db.UpdateSiteStatus(
				context.Background(), c.Device.ID, se.SiteId, se.Status, int(se.Pid),
			)
		}
	}
}

func (c *AgentConn) SendCommand(ctx context.Context, action string, payload interface{}) (*shared.Result, error) {
	cmdID, err := GenerateToken(16)
	if err != nil {
		return nil, err
	}

	var payloadBytes []byte
	if payload != nil {
		payloadBytes, err = shared.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}
	}

	env := shared.Envelope{
		Command: &shared.Command{
			Id:      cmdID,
			Action:  action,
			Payload: payloadBytes,
		},
	}
	msg, err := shared.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("marshal envelope: %w", err)
	}

	commandTimeout := wsCommandTimeout
	if action == "build_site" { commandTimeout = 2 * time.Hour }
	ch := make(chan *shared.Result, 1)
	c.pendingMu.Lock()
	c.pending[cmdID] = ch
	c.pendingMu.Unlock()

	select {
	case c.send <- msg:
	case <-c.done:
		c.pendingMu.Lock(); delete(c.pending,cmdID); c.pendingMu.Unlock()
		return nil, fmt.Errorf("agent disconnected")
	case <-ctx.Done():
		c.pendingMu.Lock(); delete(c.pending,cmdID); c.pendingMu.Unlock()
		return nil, ctx.Err()
	case <-time.After(wsWriteWait):
		c.pendingMu.Lock(); delete(c.pending,cmdID); c.pendingMu.Unlock()
		return nil, fmt.Errorf("write timeout")
	}

	select {
	case res := <-ch:
		return res, nil
	case <-c.done:
		return nil, fmt.Errorf("agent disconnected")
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(commandTimeout):
		c.pendingMu.Lock()
		delete(c.pending, cmdID)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("command timeout: %s", action)
	}
}

func (srv *Server) OnlineDevices() []map[string]interface{} {
	conns := srv.hub.List()
	out := make([]map[string]interface{}, 0, len(conns))
	for _, c := range conns {
		if c.Device == nil {
			continue
		}
		out = append(out, map[string]interface{}{
			"id":        c.Device.ID,
			"device_id": c.Device.DeviceID,
			"hostname":  c.Device.Hostname,
			"tailscale": c.Device.TailscaleIP,
			"user_id":   c.Device.UserID,
		})
	}
	return out
}
