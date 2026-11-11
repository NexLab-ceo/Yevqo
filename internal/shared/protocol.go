// Package shared — общие структуры для сервера и агента.
// Этот файл — вручную написанная замена сгенерированному protoc-коду.
package shared

import "encoding/json"

// ============================================================
// ENVELOPE — обёртка для всех сообщений
// ============================================================

type Envelope struct {
	AuthReq  *AuthRequest
	AuthResp *AuthResponse
	Command  *Command
	Result   *Result
	Event    *Event
}

// ============================================================
// АУТЕНТИФИКАЦИЯ
// ============================================================

type AuthRequest struct {
	Token        string
	DeviceId     string
	Hostname     string
	TailscaleIp  string
	AgentVersion string
}

type AuthResponse struct {
	Ok      bool
	Message string
	UserId  int64
}

// ============================================================
// КОМАНДЫ (Сервер -> Агент)
// ============================================================

type Command struct {
	Id      string
	Action  string
	Payload []byte
}

type StartSite struct {
	SiteId  string
	Path    string
	Port    uint32
	Runtime string
}

type StopSite struct {
	SiteId string
}

type GitPull struct {
	SiteId string
}

type GetLogs struct {
	SiteId string
	Lines  uint32
}

type CreateTunnel struct {
	SiteId      string
	TunnelToken string
	Hostname    string
}

// ============================================================
// РЕЗУЛЬТАТЫ (Агент -> Сервер)
// ============================================================

type Result struct {
	Id      string
	Ok      bool
	Message string
	Payload []byte
}

type StartSiteResult struct {
	Pid  uint32
	Port uint32
}

type LogsResult struct {
	Content string
}

// ============================================================
// СОБЫТИЯ (Агент -> Сервер)
// ============================================================

type Event struct {
	Event   string
	Payload []byte
}

type LogEvent struct {
	SiteId string
	Line   string
}

type StatusEvent struct {
	SiteId string
	Status string
	Pid    uint32
}

type PingEvent struct{}

// ============================================================
// МЕТОДЫ — имитация proto.Message
// ============================================================

func (m *AuthRequest) Reset()         { *m = AuthRequest{} }
func (m *AuthRequest) String() string { return "AuthRequest" }
func (m *AuthRequest) ProtoMessage()  {}

func (m *AuthResponse) Reset()         { *m = AuthResponse{} }
func (m *AuthResponse) String() string { return "AuthResponse" }
func (m *AuthResponse) ProtoMessage()  {}

func (m *Command) Reset()         { *m = Command{} }
func (m *Command) String() string { return "Command" }
func (m *Command) ProtoMessage()  {}

func (m *StartSite) Reset()         { *m = StartSite{} }
func (m *StartSite) String() string { return "StartSite" }
func (m *StartSite) ProtoMessage()  {}

func (m *StopSite) Reset()         { *m = StopSite{} }
func (m *StopSite) String() string { return "StopSite" }
func (m *StopSite) ProtoMessage()  {}

func (m *GitPull) Reset()         { *m = GitPull{} }
func (m *GitPull) String() string { return "GitPull" }
func (m *GitPull) ProtoMessage()  {}

func (m *GetLogs) Reset()         { *m = GetLogs{} }
func (m *GetLogs) String() string { return "GetLogs" }
func (m *GetLogs) ProtoMessage()  {}

func (m *CreateTunnel) Reset()         { *m = CreateTunnel{} }
func (m *CreateTunnel) String() string { return "CreateTunnel" }
func (m *CreateTunnel) ProtoMessage()  {}

func (m *Result) Reset()         { *m = Result{} }
func (m *Result) String() string { return "Result" }
func (m *Result) ProtoMessage()  {}

func (m *StartSiteResult) Reset()         { *m = StartSiteResult{} }
func (m *StartSiteResult) String() string { return "StartSiteResult" }
func (m *StartSiteResult) ProtoMessage()  {}

func (m *LogsResult) Reset()         { *m = LogsResult{} }
func (m *LogsResult) String() string { return "LogsResult" }
func (m *LogsResult) ProtoMessage()  {}

func (m *Event) Reset()         { *m = Event{} }
func (m *Event) String() string { return "Event" }
func (m *Event) ProtoMessage()  {}

func (m *LogEvent) Reset()         { *m = LogEvent{} }
func (m *LogEvent) String() string { return "LogEvent" }
func (m *LogEvent) ProtoMessage()  {}

func (m *StatusEvent) Reset()         { *m = StatusEvent{} }
func (m *StatusEvent) String() string { return "StatusEvent" }
func (m *StatusEvent) ProtoMessage()  {}

func (m *PingEvent) Reset()         { *m = PingEvent{} }
func (m *PingEvent) String() string { return "PingEvent" }
func (m *PingEvent) ProtoMessage()  {}

// ============================================================
// СЕРИАЛИЗАЦИЯ (JSON вместо protobuf)
// ============================================================

func Marshal(m interface{}) ([]byte, error) {
	return json.Marshal(m)
}

func Unmarshal(data []byte, m interface{}) error {
	return json.Unmarshal(data, m)
}

// ============================================================
// ENVELOPE — определение типа
// ============================================================

func (e *Envelope) GetType() string {
	switch {
	case e.AuthReq != nil:
		return "auth_req"
	case e.AuthResp != nil:
		return "auth_resp"
	case e.Command != nil:
		return "command"
	case e.Result != nil:
		return "result"
	case e.Event != nil:
		return "event"
	default:
		return ""
	}
}
