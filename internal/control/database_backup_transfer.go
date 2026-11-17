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

func (s *Server) handleDatabaseBackupChunk(w http.ResponseWriter,r *http.Request){
 if r.Method!=http.MethodGet{http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return}
 u:=UserFromContext(r.Context());parts:=strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/backups/"),"/"),"/")
 if len(parts)!=2||parts[1]!="chunk"{apiError(w,shared.ErrBadRequest("expected backup id/chunk"));return}
 id,err:=strconv.ParseInt(parts[0],10,64);if err!=nil||id<1{apiError(w,shared.ErrBadRequest("invalid backup id"));return}
 offset,err:=strconv.ParseInt(r.URL.Query().Get("offset"),10,64);if err!=nil||offset<0{apiError(w,shared.ErrBadRequest("invalid chunk offset"));return}
 length:=256*1024;if raw:=r.URL.Query().Get("length");raw!=""{length,err=strconv.Atoi(raw);if err!=nil||length<1||length>256*1024{apiError(w,shared.ErrBadRequest("chunk length must be 1–262144 bytes"));return}}
 var deviceID int64;var serverID sql.NullInt64;var fileName,status,resourceType,resourceID string;var size int64
 err=s.db.conn.QueryRowContext(r.Context(),`SELECT device_id,database_server_id,file_name,size_bytes,status,resource_type,resource_id FROM backups WHERE id=? AND user_id=?`,id,u.ID).Scan(&deviceID,&serverID,&fileName,&size,&status,&resourceType,&resourceID)
 if err==sql.ErrNoRows{apiError(w,shared.ErrNotFound("backup not found"));return};if err!=nil{apiError(w,shared.ErrInternal("could not load backup metadata"));return}
 if size<1 { apiError(w,shared.ErrBadRequest("backup is empty"));return }
 conn,online:=s.hub.Get(deviceID);if !online{apiError(w,shared.ErrBadRequest("backup device is offline"));return}
 ctx,cancel:=context.WithTimeout(r.Context(),20*time.Second);defer cancel()
 var result *shared.Result
 if resourceType=="database" && status=="verified" && serverID.Valid && serverID.Int64>0 {
  result,err=conn.SendCommand(ctx,"read_database_backup_chunk",&shared.ReadDatabaseBackupChunk{ServerID:serverID.Int64,FileName:fileName,Offset:offset,Length:length})
 } else if resourceType=="site" && status=="completed" {
  target:=strings.SplitN(resourceID,":",2)
  if len(target)!=2 || target[0]!=strconv.FormatInt(deviceID,10) || !validBackupSiteID(target[1]) { apiError(w,shared.ErrBadRequest("invalid site backup scope"));return }
  result,err=conn.SendCommand(ctx,"read_site_backup_chunk",&shared.ReadSiteBackupChunk{SiteId:target[1],FileName:fileName,Offset:offset,Length:length})
 } else { apiError(w,shared.ErrBadRequest("backup is not available for download"));return }
 if err!=nil{apiError(w,shared.ErrInternal("Agent backup transfer command failed"));return};if !result.Ok{apiError(w,shared.ErrBadRequest(result.Message));return}
 var chunk shared.DatabaseBackupChunk;if err:=shared.Unmarshal(result.Payload,&chunk);err!=nil{apiError(w,shared.ErrInternal("invalid backup chunk response"));return}
 if chunk.TotalSize!=size||chunk.Offset!=offset||len(chunk.Data)>length{apiError(w,shared.ErrInternal("backup chunk metadata mismatch"));return}
 writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"data":chunk.Data,"offset":chunk.Offset,"total_size":chunk.TotalSize,"eof":chunk.EOF})
}
