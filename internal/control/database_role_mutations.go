package control

import (
	"context"
	"database/sql"
	"net/http"
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
	ownerID,_,accessErr:=s.authorizeDatabaseServer(r.Context(),serverID,u.ID,"database.manage");if accessErr!=nil{apiError(w,accessErr);return}
	if err := decodeJSON(r, &in); err != nil {
		apiError(w, shared.ErrBadRequest("invalid role mutation JSON"))
		return
	}
	in.Action = strings.TrimSpace(in.Action)
	in.Name = strings.TrimSpace(in.Name)
	if in.Action != "create" && in.Action != "drop" && in.Action != "password" {
		apiError(w, shared.ErrBadRequest("action must be create, drop or password"))
		return
	}
	if len(in.Name) < 1 || len(in.Name) > 63 {
		apiError(w, shared.ErrBadRequest("role name must be 1–63 characters"))
		return
	}
	if (in.Action == "create" || in.Action == "password") && (len(in.Password) < 12 || len(in.Password) > 256) {
		apiError(w, shared.ErrBadRequest("password must be 12–256 characters"))
		return
	}
	if in.Action == "drop" && in.Password != "" {
		apiError(w, shared.ErrBadRequest("drop request must not include a password"))
		return
	}
	var deviceID int64
	var engine, status, dataDir string
	var port int
	err := s.db.conn.QueryRowContext(r.Context(), `SELECT device_id,engine,status,port,data_directory FROM database_servers WHERE id=? AND user_id=?`, serverID, ownerID).Scan(&deviceID, &engine, &status, &port, &dataDir)
	if err == sql.ErrNoRows {
		apiError(w, shared.ErrNotFound("database server not found"))
		return
	}
	if err != nil {
		apiError(w, shared.ErrInternal("could not load database server"))
		return
	}
	if (engine != "postgresql" && engine != "mariadb" && engine != "mysql") || status != "ready" {
		apiError(w, shared.ErrBadRequest("a supported ready database server is required"))
		return
	}
	conn, online := s.hub.Get(deviceID)
	if !online {
		apiError(w, shared.ErrBadRequest("device is offline"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	req := &shared.ManageDatabaseRole{Engine:engine,DataDirectory:dataDir,ServerID: serverID, Port: uint32(port), Name: in.Name, Password: in.Password}
	result, err := conn.SendCommand(ctx, in.Action+"_database_role", req)
	if err != nil {
		apiError(w, shared.ErrInternal("Agent database user command failed"))
		return
	}
	if !result.Ok {
		apiError(w, shared.ErrBadRequest(result.Message))
		return
	}
	_ = s.db.Log(context.Background(), &shared.LogEntry{UserID: shared.PtrInt64(u.ID), DeviceID: shared.PtrInt64(deviceID), Action: "database_role_"+in.Action, Details: map[string]interface{}{"server_id": serverID, "role": in.Name}})
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": engine+" user "+in.Action+" completed"})
}
