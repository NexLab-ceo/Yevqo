package control

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"veltrix/internal/shared"
)

func (s *Server) handleRestoreSiteBackup(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost{http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return}
	u:=UserFromContext(r.Context())
	var in struct{BackupID int64 `json:"backup_id"`}
	ownerID,_,accessErr:=s.authorizeBackup(r.Context(),in.BackupID,u.ID,"backup.manage");if accessErr!=nil{apiError(w,accessErr);return}
	if err:=decodeJSON(r,&in);err!=nil||in.BackupID<1{apiError(w,shared.ErrBadRequest("invalid site restore request"));return}
	var deviceID int64;var fileName,status,resourceType,resourceID string;var size int64
	err:=s.db.conn.QueryRowContext(r.Context(),`SELECT device_id,file_name,status,resource_type,resource_id,size_bytes FROM backups WHERE id=? AND user_id=?`,in.BackupID,ownerID).Scan(&deviceID,&fileName,&status,&resourceType,&resourceID,&size)
	if err==sql.ErrNoRows{apiError(w,shared.ErrNotFound("site backup not found"));return}
	if err!=nil{apiError(w,shared.ErrInternal("could not load site backup"));return}
	if resourceType!="site"||status!="completed"||size<1{apiError(w,shared.ErrBadRequest("only completed website backups can be restored"));return}
	parts:=strings.SplitN(resourceID,":",2)
	if len(parts)!=2||parts[0]!=strconv.FormatInt(deviceID,10)||!validBackupSiteID(parts[1]){apiError(w,shared.ErrBadRequest("invalid website backup scope"));return}
	var owned int
	if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM sites st JOIN devices d ON d.id=st.device_id WHERE st.device_id=? AND st.site_id=? AND d.user_id=?`,deviceID,parts[1],ownerID).Scan(&owned);err!=nil||owned!=1{apiError(w,shared.ErrNotFound("target website no longer exists"));return}
	conn,online:=s.hub.Get(deviceID);if !online{apiError(w,shared.ErrBadRequest("source device is offline"));return}
	ctx,cancel:=context.WithTimeout(r.Context(),5*time.Minute);defer cancel()
	result,err:=conn.SendCommand(ctx,"restore_site_backup",&shared.RestoreSiteBackup{SiteId:parts[1],FileName:fileName})
	if err!=nil{apiError(w,shared.ErrInternal("site restore command failed"));return}
	if !result.Ok{apiError(w,shared.ErrBadRequest(result.Message));return}
	_,_ = s.db.conn.ExecContext(r.Context(),`UPDATE sites SET status='stopped',pid=0 WHERE device_id=? AND site_id=?`,deviceID,parts[1])
	_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),DeviceID:shared.PtrInt64(deviceID),Action:"site_backup_restored",Details:map[string]interface{}{"backup_id":in.BackupID,"site_id":parts[1]}})
	_,_ = s.db.conn.ExecContext(r.Context(),`INSERT INTO notifications(user_id,type,title,body) VALUES(?,?,?,?)`,u.ID,"success","Website backup restored",fmt.Sprintf("Backup %s was restored to website %s. The site is stopped; start it after checking the restored files.",fileName,parts[1]))
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"site_id":parts[1],"status":"stopped","message":"Backup restored. The website is stopped and can be started after verification."})
}
