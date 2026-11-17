package agent

import "veltrix/internal/shared"

func (h *CommandHandler) handleReadDatabaseBackupChunk(cmd *shared.Command)*shared.Result{
 var req shared.ReadDatabaseBackupChunk;if err:=shared.Unmarshal(cmd.Payload,&req);err!=nil{return errorResult(cmd.Id,"invalid backup chunk payload")}
 chunk,err:=readDatabaseBackupChunk(req);if err!=nil{return errorResult(cmd.Id,err.Error())}
 payload,err:=shared.Marshal(chunk);if err!=nil{return errorResult(cmd.Id,"could not encode backup chunk")}
 return &shared.Result{Id:cmd.Id,Ok:true,Message:"backup chunk read",Payload:payload}
}

func (h *CommandHandler) handleReadSiteBackupChunk(cmd *shared.Command) *shared.Result {
	var req shared.ReadSiteBackupChunk
	if err:=shared.Unmarshal(cmd.Payload,&req);err!=nil{return errorResult(cmd.Id,"invalid read_site_backup_chunk payload")}
	chunk,err:=readSiteBackupChunk(req);if err!=nil{return errorResult(cmd.Id,err.Error())}
	payload,err:=shared.Marshal(chunk);if err!=nil{return errorResult(cmd.Id,"could not encode site backup chunk")}
	return &shared.Result{Id:cmd.Id,Ok:true,Message:"site backup chunk read",Payload:payload}
}

func (h *CommandHandler) handleRestoreSiteBackup(cmd *shared.Command) *shared.Result {
	var req shared.RestoreSiteBackup
	if err:=shared.Unmarshal(cmd.Payload,&req);err!=nil{return errorResult(cmd.Id,"invalid restore_site_backup payload")}
	if err:=h.processes.RestoreSiteBackup(req);err!=nil{return errorResult(cmd.Id,err.Error())}
	return &shared.Result{Id:cmd.Id,Ok:true,Message:"site backup restored; site is stopped and ready to start"}
}
