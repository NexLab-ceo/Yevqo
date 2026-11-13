package control

import (
 "context"
 "database/sql"
 "net/http"
 "strings"
 "time"
 "veltrix/internal/shared"
)

type databasePrivilegeInput struct {
 Action string `json:"action"`
 Database string `json:"database"`
 Role string `json:"role"`
 Privilege string `json:"privilege"`
 Scope string `json:"scope"`
 Schema string `json:"schema"`
}

func (s *Server) handleDatabaseServerPrivileges(w http.ResponseWriter,r *http.Request,serverID int64){
 u:=UserFromContext(r.Context());var in databasePrivilegeInput
 if err:=decodeJSON(r,&in);err!=nil{apiError(w,shared.ErrBadRequest("invalid privilege JSON"));return}
 in.Action=strings.TrimSpace(in.Action);in.Database=strings.TrimSpace(in.Database);in.Role=strings.TrimSpace(in.Role);in.Privilege=strings.ToUpper(strings.TrimSpace(in.Privilege))
 if in.Action!="grant"&&in.Action!="revoke"{apiError(w,shared.ErrBadRequest("action must be grant or revoke"));return}
 if len(in.Database)<1||len(in.Database)>63||len(in.Role)<1||len(in.Role)>63{apiError(w,shared.ErrBadRequest("database and role names must be 1–63 characters"));return}
 if in.Scope==""{in.Scope="database"}
 if in.Scope=="database" {if in.Privilege!="CONNECT"&&in.Privilege!="CREATE"&&in.Privilege!="TEMPORARY"{apiError(w,shared.ErrBadRequest("database privilege must be CONNECT, CREATE or TEMPORARY"));return};if in.Schema!=""{apiError(w,shared.ErrBadRequest("schema must be empty for database scope"));return}}
 else if in.Scope=="schema" {if len(in.Schema)<1||len(in.Schema)>63||!postgresIdentifier.MatchString(in.Schema){apiError(w,shared.ErrBadRequest("invalid schema name"));return};if in.Privilege!="USAGE"&&in.Privilege!="CREATE"{apiError(w,shared.ErrBadRequest("schema privilege must be USAGE or CREATE"));return}}
 else{apiError(w,shared.ErrBadRequest("scope must be database or schema"));return}
 var deviceID int64;var engine,status string;var port int
 err:=s.db.conn.QueryRowContext(r.Context(),`SELECT device_id,engine,status,port FROM database_servers WHERE id=? AND user_id=?`,serverID,u.ID).Scan(&deviceID,&engine,&status,&port)
 if err==sql.ErrNoRows{apiError(w,shared.ErrNotFound("database server not found"));return};if err!=nil{apiError(w,shared.ErrInternal("could not load database server"));return}
 if engine!="postgresql"||status!="ready"{apiError(w,shared.ErrBadRequest("a ready PostgreSQL server is required"));return}
 // Verify both objects exist before executing a privilege mutation.
 conn,online:=s.hub.Get(deviceID);if !online{apiError(w,shared.ErrBadRequest("device is offline"));return}
 ctx,cancel:=context.WithTimeout(r.Context(),25*time.Second);defer cancel()
 req:=&shared.ManageDatabasePrivilege{ServerID:serverID,Port:uint32(port),Database:in.Database,Role:in.Role,Privilege:in.Privilege,Scope:in.Scope,Schema:in.Schema,Action:in.Action}
 result,err:=conn.SendCommand(ctx,"manage_database_privilege",req);if err!=nil{apiError(w,shared.ErrInternal("Agent database privilege command failed"));return};if !result.Ok{apiError(w,shared.ErrBadRequest(result.Message));return}
 _=s.db.Log(context.Background(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),DeviceID:shared.PtrInt64(deviceID),Action:"database_privilege_"+in.Action,Details:map[string]interface{}{"server_id":serverID,"database":in.Database,"role":in.Role,"privilege":in.Privilege,"scope":in.Scope,"schema":in.Schema}})
 writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"message":"Database privilege "+in.Action+" completed"})
}
