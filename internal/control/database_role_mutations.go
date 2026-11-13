package control

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"veltrix/internal/shared"
)

type databaseRoleMutationInput struct {
	Action string `json:"action"`
	Name string `json:"name"`
	Password string `json:"password"`
}

func (s *Server) handleDatabaseServerRoleMutation(w http.ResponseWriter, r *http.Request, serverID int64) {
	u := UserFromContext(r.Context())
	var in databaseRoleMutationInput
	if err := decodeJSON(r, &in); err != nil {
		apiError(w, shared.ErrBadRequest("invalid role mutation JSON"))
		return
	}
	in.Action = strings.TrimSpace(in.Action)
	in.Name = strings.TrimSpace(in.Name)
	if in.Action != "create" && in.Action != "drop" {
		apiError(w, shared.ErrBadRequest("action must be create or drop"))
		return
	}
	if len(in.Name) < 1 || len(in.Name) > 63 {
		apiError(w, shared.ErrBadRequest("role name must be 1–63 characters"))
		return
	}
	if in.Action == "create" && (len(in.Password) < 12 || len(in.Password) > 256) {
		apiError(w, shared.ErrBadRequest("password must be 12–256 characters"))
		return
	}
	if in.Action == "drop" && in.Password != "" {
		apiError(w, shared.ErrBadRequest("drop request must not include a password"))
		return
	}
	var deviceID int64
	var engine, status string
	var port int
	err := s.db.conn.QueryRowContext(r.Context(), `SELECT device_id,engine,status,port FROM database_servers WHERE id=? AND user_id=?`, serverID, u.ID).Scan(&deviceID, &engine, &status, &port)
	if err == sql.ErrNoRows {
		apiError(w, shared.ErrNotFound("database server not found"))
		return
	}
	if err != nil {
		apiError(w, shared.ErrInternal("could not load database server"))
		return
	}
	if engine != "postgresql" || status != "ready" {
		apiError(w, shared.ErrBadRequest("a ready PostgreSQL server is required"))
		return
	}
	conn, online := s.hub.Get(deviceID)
	if !online {
		apiError(w, shared.ErrBadRequest("device is offline"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	req := &shared.ManageDatabaseRole{ServerID: serverID, Port: uint32(port), Name: in.Name, Password: in.Password}
	result, err := conn.SendCommand(ctx, in.Action+"_database_role", req)
	if err != nil {
		apiError(w, shared.ErrInternal("Agent PostgreSQL role command failed"))
		return
	}
	if !result.Ok {
		apiError(w, shared.ErrBadRequest(result.Message))
		return
	}
	_ = s.db.Log(context.Background(), &shared.LogEntry{UserID: shared.PtrInt64(u.ID), DeviceID: shared.PtrInt64(deviceID), Action: "database_role_"+in.Action, Details: map[string]interface{}{"server_id": serverID, "role": in.Name}})
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "PostgreSQL role "+in.Action+" completed"})
}
