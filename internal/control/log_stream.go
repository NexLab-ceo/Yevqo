package control

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"veltrix/internal/shared"
)

type logReplayEntry struct { userID, deviceID int64; siteID string; seq uint64; payload []byte }

type logStreamSubscriber struct {
	userID   int64
	deviceID int64
	siteID   string
	messages chan []byte
	lastSeq uint64
}

type logStreamHub struct {
	mu          sync.Mutex
	subscribers map[*logStreamSubscriber]struct{}
	history []logReplayEntry
	sequence map[string]uint64
}

func newLogStreamHub() *logStreamHub {
	return &logStreamHub{subscribers: make(map[*logStreamSubscriber]struct{}), sequence: make(map[string]uint64)}
}

func logStreamKey(userID, deviceID int64, siteID string) string { return strconv.FormatInt(userID,10)+":"+strconv.FormatInt(deviceID,10)+":"+siteID }

func (h *logStreamHub) subscribe(userID, deviceID int64, siteID string, since uint64) *logStreamSubscriber {
	sub := &logStreamSubscriber{userID:userID, deviceID:deviceID, siteID:siteID, messages:make(chan []byte,128), lastSeq:since}
	h.mu.Lock()
	h.subscribers[sub]=struct{}{}
	for _, entry := range h.history { if entry.userID==userID && entry.deviceID==deviceID && entry.siteID==siteID && entry.seq>since { select { case sub.messages<-entry.payload: default: } } }
	h.mu.Unlock()
	return sub
}

func (h *logStreamHub) unsubscribe(sub *logStreamSubscriber) {
	h.mu.Lock()
	delete(h.subscribers, sub)
	h.mu.Unlock()
}

func (h *logStreamHub) publishAgentStatus(userID, deviceID int64, status string) {
	payload, err := json.Marshal(map[string]interface{}{
		"type": "agent_status", "device_id": deviceID, "status": status,
		"timestamp": time.Now().UTC(),
	})
	if err != nil { return }
	h.mu.Lock()
	defer h.mu.Unlock()
	for sub := range h.subscribers {
		if sub.userID != userID || sub.deviceID != deviceID { continue }
		select {
		case sub.messages <- payload:
		default:
			select { case <-sub.messages: default: }
			select { case sub.messages <- payload: default: }
		}
	}
}

func (h *logStreamHub) publish(userID, deviceID int64, event shared.LogEvent) {
 key:=logStreamKey(userID,deviceID,event.SiteId); h.mu.Lock(); h.sequence[key]++; seq:=h.sequence[key]
 payload,err:=json.Marshal(map[string]interface{}{"type":"log","device_id":deviceID,"site_id":event.SiteId,"line":event.Line,"timestamp":time.Now().UTC(),"seq":seq}); if err!=nil {h.mu.Unlock();return}
 h.history=append(h.history,logReplayEntry{userID:userID,deviceID:deviceID,siteID:event.SiteId,seq:seq,payload:payload}); if len(h.history)>2000 {h.history=append([]logReplayEntry(nil),h.history[len(h.history)-2000:]...)}
 for sub:=range h.subscribers {if sub.userID!=userID||sub.deviceID!=deviceID||sub.siteID!=event.SiteId {continue};select{case sub.messages<-payload:default:select{case <-sub.messages:default:};select{case sub.messages<-payload:default:}}};h.mu.Unlock()
}
func validLogSiteID(value string) bool {
	if len(value) == 0 || len(value) > 64 { return false }
	for _, r := range value {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

// handleLogStream authenticates the browser session and verifies device ownership
// before upgrading the connection. A stream can only subscribe to one owned site.
func (s *Server) handleLogStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	user := UserFromContext(r.Context())
	if user == nil { http.Error(w, "unauthorized", http.StatusUnauthorized); return }

	deviceID, err := strconv.ParseInt(r.URL.Query().Get("device_id"), 10, 64)
	siteID := r.URL.Query().Get("site_id")
	if err != nil || deviceID <= 0 || !validLogSiteID(siteID) {
		http.Error(w, "invalid device or site", http.StatusBadRequest)
		return
	}
	device, err := s.db.GetDeviceByID(r.Context(), deviceID)
	if err != nil || device.UserID != user.ID {
		http.Error(w, "device not found", http.StatusNotFound)
		return
	}
	sites, err := s.db.ListSitesByDevice(r.Context(), deviceID)
	if err != nil { http.Error(w, "unable to verify site", http.StatusInternalServerError); return }
	found := false
	for _, site := range sites { if site.SiteID == siteID { found = true; break } }
	if !found { http.Error(w, "site not found", http.StatusNotFound); return }

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil { return }
	defer conn.Close()
	conn.SetReadLimit(1024)
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(60 * time.Second)) })

	since, _ := strconv.ParseUint(r.URL.Query().Get("since"), 10, 64)
	sub := s.logStreams.subscribe(user.ID, deviceID, siteID, since)
	defer s.logStreams.unsubscribe(sub)

	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			if _, _, err := conn.ReadMessage(); err != nil { return }
		}
	}()

	for {
		select {
		case <-readDone:
			return
		case message := <-sub.messages:
			_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil { return }
		case <-ping.C:
			_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil { return }
		}
	}
}
