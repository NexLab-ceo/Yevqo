package control

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"veltrix/internal/shared"
)

func (s *Server) handleBackupTransfer(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost{http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return}
	u:=UserFromContext(r.Context())
	parts:=strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/backups/"),"/"),"/")
	if len(parts)!=2||parts[1]!="transfer"{apiError(w,shared.ErrBadRequest("expected backup id/transfer"));return}
	backupID,err:=strconv.ParseInt(parts[0],10,64);if err!=nil||backupID<1{apiError(w,shared.ErrBadRequest("invalid backup id"));return}
	ownerID,_,accessErr:=s.authorizeBackup(r.Context(),backupID,u.ID,"backup.manage");if accessErr!=nil{apiError(w,accessErr);return}
	var in struct{TargetDeviceID int64 `json:"target_device_id"`;TargetSiteID string `json:"target_site_id"`;TargetServerID int64 `json:"target_server_id"`}
	if err:=decodeJSON(r,&in);err!=nil{apiError(w,shared.ErrBadRequest("invalid backup transfer request"));return}
	var sourceDeviceID int64;var sourceServerID sql.NullInt64;var fileName,status,resourceType,resourceID string;var size int64
	err=s.db.conn.QueryRowContext(r.Context(),`SELECT device_id,database_server_id,file_name,status,resource_type,resource_id,size_bytes FROM backups WHERE id=? AND user_id=?`,backupID,ownerID).Scan(&sourceDeviceID,&sourceServerID,&fileName,&status,&resourceType,&resourceID,&size)
	if err==sql.ErrNoRows{apiError(w,shared.ErrNotFound("backup not found"));return};if err!=nil{apiError(w,shared.ErrInternal("could not load source backup"));return}
	if size<1||size>2<<30||((resourceType=="site"&&status!="completed")||(resourceType=="database"&&status!="verified")){apiError(w,shared.ErrBadRequest("only completed or verified backups can be transferred"));return}
	sourceSiteID:=""
	if resourceType=="site"{target:=strings.SplitN(resourceID,":",2);if len(target)!=2||target[0]!=strconv.FormatInt(sourceDeviceID,10)||!validBackupSiteID(target[1]){apiError(w,shared.ErrBadRequest("invalid source website backup"));return};sourceSiteID=target[1]}
	if resourceType!="site"&&resourceType!="database"{apiError(w,shared.ErrBadRequest("unsupported backup type"));return}
	if resourceType=="database"&&(!sourceServerID.Valid||sourceServerID.Int64<1){apiError(w,shared.ErrBadRequest("source database backup server is unavailable"));return}
	sourceConn,sourceOnline:=s.hub.Get(sourceDeviceID);if !sourceOnline{apiError(w,shared.ErrBadRequest("source device is offline"));return}
	targetDeviceID:=in.TargetDeviceID;targetSiteID:="";targetServerID:=int64(0);targetFileName:=fileName
	if resourceType=="site"{
		targetSiteID=strings.TrimSpace(in.TargetSiteID)
		if targetDeviceID<1||!validBackupSiteID(targetSiteID)||in.TargetServerID!=0{apiError(w,shared.ErrBadRequest("choose a target website"));return}
		if targetDeviceID==sourceDeviceID&&targetSiteID==sourceSiteID{apiError(w,shared.ErrBadRequest("source and target website are the same"));return}
		var owned int
		if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM sites st JOIN devices d ON d.id=st.device_id WHERE st.device_id=? AND st.site_id=? AND d.user_id=?`,targetDeviceID,targetSiteID,ownerID).Scan(&owned);err!=nil||owned!=1{apiError(w,shared.ErrNotFound("target website not found"));return}
		suffix:=strings.TrimPrefix(fileName,sourceSiteID+"-");if suffix==fileName{apiError(w,shared.ErrBadRequest("invalid source archive name"));return}
		targetFileName=targetSiteID+"-"+suffix
	}else{
		if in.TargetServerID<1||in.TargetDeviceID!=0||in.TargetSiteID!=""{apiError(w,shared.ErrBadRequest("choose a target PostgreSQL server"));return}
		var engine,serverStatus string
		if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT device_id,engine,status FROM database_servers WHERE id=? AND user_id=?`,in.TargetServerID,ownerID).Scan(&targetDeviceID,&engine,&serverStatus);err!=nil||engine!="postgresql"||serverStatus!="ready"{apiError(w,shared.ErrNotFound("ready target PostgreSQL server not found"));return}
		targetServerID=in.TargetServerID
		targetFileName=fmt.Sprintf("transfer-%d.dump",backupID)
	}
	targetConn,targetOnline:=s.hub.Get(targetDeviceID);if !targetOnline{apiError(w,shared.ErrBadRequest("target device is offline"));return}
	var newBackupID int64
	if resourceType=="site"{
		targetResourceID:=fmt.Sprintf("%d:%s",targetDeviceID,targetSiteID)
		res,err:=s.db.conn.ExecContext(r.Context(),`INSERT INTO backups(user_id,device_id,resource_type,resource_id,file_name,size_bytes,status) VALUES(?,?, 'site', ?, ?,?,'queued')`,ownerID,targetDeviceID,targetResourceID,targetFileName,size)
		if err!=nil{apiError(w,shared.ErrInternal("could not create target backup record"));return};newBackupID,_=res.LastInsertId()
	}else{
		res,err:=s.db.conn.ExecContext(r.Context(),`INSERT INTO backups(user_id,device_id,database_server_id,resource_type,resource_id,file_name,size_bytes,status) VALUES(?,?,?,'database',?,?,?,'queued')`,ownerID,targetDeviceID,targetServerID,fmt.Sprintf("transfer:%d",backupID),targetFileName,size)
		if err!=nil{apiError(w,shared.ErrInternal("could not create target backup record"));return};newBackupID,_=res.LastInsertId()
	}
	go s.runBackupTransfer(backupID,newBackupID,ownerID,u.ID,sourceConn,targetConn,sourceDeviceID,targetDeviceID,resourceType,sourceSiteID,targetSiteID,sourceServerID.Int64,targetServerID,fileName,targetFileName,size)
	writeJSON(w,http.StatusAccepted,map[string]interface{}{"ok":true,"backup_id":newBackupID,"status":"queued","target_device_id":targetDeviceID,"file_name":targetFileName})
}

func (s *Server) runBackupTransfer(sourceID,targetID,ownerID,actorID int64,sourceConn,targetConn *AgentConn,sourceDevice,targetDevice int64,resourceType,sourceSite,targetSite string,sourceServer,targetServer int64,sourceFile,targetFile string,total int64){
	_,_ = s.db.conn.ExecContext(context.Background(),`UPDATE backups SET status='running' WHERE id=? AND user_id=?`,targetID,ownerID)
	var transferBytes [16]byte;if _,err:=rand.Read(transferBytes[:]);err!=nil{s.failBackupTransfer(targetID,ownerID,actorID,"could not initialize transfer");return}
	transferID:=hex.EncodeToString(transferBytes[:])
	offset:=int64(0)
	for offset<total{
		length:=256*1024
		var readCommand string;var readPayload interface{}
		if resourceType=="site"{readCommand="read_site_backup_chunk";readPayload=&shared.ReadSiteBackupChunk{SiteId:sourceSite,FileName:sourceFile,Offset:offset,Length:length}}else{readCommand="read_database_backup_chunk";readPayload=&shared.ReadDatabaseBackupChunk{ServerID:sourceServer,FileName:sourceFile,Offset:offset,Length:length}}
		ctx,cancel:=context.WithTimeout(context.Background(),45*time.Second)
		readResult,err:=sourceConn.SendCommand(ctx,readCommand,readPayload)
		cancel()
		if err!=nil||readResult==nil||!readResult.Ok{s.abortBackupTransfer(targetConn,transferID,resourceType,targetSite,targetServer,targetFile);s.failBackupTransfer(targetID,ownerID,actorID,"source backup read failed");return}
		var chunk shared.DatabaseBackupChunk
		if err:=shared.Unmarshal(readResult.Payload,&chunk);err!=nil||chunk.Offset!=offset||chunk.TotalSize!=total||len(chunk.Data)==0||len(chunk.Data)>length||offset+int64(len(chunk.Data))>total||chunk.EOF!=(offset+int64(len(chunk.Data))==total){s.abortBackupTransfer(targetConn,transferID,resourceType,targetSite,targetServer,targetFile);s.failBackupTransfer(targetID,ownerID,actorID,"source backup chunk validation failed");return}
		final:=offset+int64(len(chunk.Data))==total
		writeReq:=&shared.WriteBackupChunk{TransferID:transferID,ResourceType:resourceType,SiteId:targetSite,ServerID:targetServer,FileName:targetFile,Offset:offset,TotalSize:total,Data:chunk.Data,Final:final}
		ctx,cancel=context.WithTimeout(context.Background(),45*time.Second)
		writeResult,writeErr:=targetConn.SendCommand(ctx,"write_backup_chunk",writeReq)
		cancel()
		if writeErr!=nil||writeResult==nil||!writeResult.Ok{s.abortBackupTransfer(targetConn,transferID,resourceType,targetSite,targetServer,targetFile);s.failBackupTransfer(targetID,ownerID,actorID,"target backup write failed");return}
		var accepted shared.WriteBackupChunkResult
		if err:=shared.Unmarshal(writeResult.Payload,&accepted);err!=nil||accepted.Received!=offset+int64(len(chunk.Data))||(final&&!accepted.Complete){s.abortBackupTransfer(targetConn,transferID,resourceType,targetSite,targetServer,targetFile);s.failBackupTransfer(targetID,ownerID,actorID,"target backup chunk acknowledgement was invalid");return}
		offset=accepted.Received
	}
	finalStatus:="verified";if resourceType=="site"{finalStatus="completed"}
	_,_ = s.db.conn.ExecContext(context.Background(),`UPDATE backups SET status=? WHERE id=? AND user_id=?`,finalStatus,targetID,ownerID)
	s.notifyWorkspaceEvent(context.Background(),ownerID,actorID,"success","Backup transfer completed",fmt.Sprintf("Backup #%d was copied to device %d and verified.",sourceID,targetDevice))
	_ = s.db.Log(context.Background(),&shared.LogEntry{UserID:shared.PtrInt64(actorID),DeviceID:shared.PtrInt64(targetDevice),Action:"backup_transferred",Details:map[string]interface{}{"source_backup_id":sourceID,"backup_id":targetID,"resource_type":resourceType,"size_bytes":total}})
}

func (s *Server) abortBackupTransfer(conn *AgentConn,id,resourceType,site string,server int64,file string){
	ctx,cancel:=context.WithTimeout(context.Background(),10*time.Second);defer cancel()
	_,_ = conn.SendCommand(ctx,"write_backup_chunk",&shared.WriteBackupChunk{TransferID:id,ResourceType:resourceType,SiteId:site,ServerID:server,FileName:file,Abort:true})
}
func (s *Server) failBackupTransfer(id,ownerID,actorID int64,reason string){
	_,_ = s.db.conn.ExecContext(context.Background(),`UPDATE backups SET status='failed' WHERE id=? AND user_id=?`,id,ownerID)
	_ = s.db.Log(context.Background(),&shared.LogEntry{UserID:shared.PtrInt64(actorID),WorkspaceOwnerID:shared.PtrInt64(ownerID),Action:"backup_transfer_failed",Details:map[string]interface{}{"ok":false,"backup_id":id,"error":reason}})
	s.notifyWorkspaceEvent(context.Background(),ownerID,actorID,"error","Backup transfer failed",reason)
}
