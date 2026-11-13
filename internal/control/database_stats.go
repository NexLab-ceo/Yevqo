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

func (s *Server) handleDatabaseStats(w http.ResponseWriter,r *http.Request){
 u:=UserFromContext(r.Context());raw:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/databases/"),"/");id,err:=strconv.ParseInt(raw,10,64)
 if err!=nil||id<1{apiError(w,shared.ErrBadRequest("invalid database id"));return}
 var dbName,status,serverStatus,engine string;var serverID,deviceID int64;var port int
 err=s.db.conn.QueryRowContext(r.Context(),`SELECT d.name,d.status,ds.id,ds.device_id,ds.port,ds.status,ds.engine FROM databases d JOIN database_servers ds ON ds.id=d.server_id WHERE d.id=? AND d.user_id=?`,id,u.ID).Scan(&dbName,&status,&serverID,&deviceID,&port,&serverStatus,&engine)
 if err==sql.ErrNoRows{apiError(w,shared.ErrNotFound("database not found"));return};if err!=nil{apiError(w,shared.ErrInternal("could not load database stats target"));return}
 if status!="ready"||serverStatus!="ready"||engine!="postgresql"{apiError(w,shared.ErrBadRequest("database and PostgreSQL server must be ready"));return}
 conn,online:=s.hub.Get(deviceID);if !online{apiError(w,shared.ErrBadRequest("device is offline"));return}
 ctx,cancel:=context.WithTimeout(r.Context(),20*time.Second);defer cancel()
 result,err:=conn.SendCommand(ctx,"get_database_stats",&shared.GetDatabaseStats{ServerID:serverID,Port:uint32(port),Database:dbName})
 if err!=nil{apiError(w,shared.ErrInternal("Agent database stats command failed"));return};if !result.Ok{apiError(w,shared.ErrBadRequest(result.Message));return}
 var stats shared.DatabaseStats;if err:=shared.Unmarshal(result.Payload,&stats);err!=nil{apiError(w,shared.ErrInternal("invalid database stats response"));return}
 writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"stats":stats})
}
