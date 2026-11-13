package agent

import "veltrix/internal/shared"

func (h *CommandHandler) handleManageDatabasePrivilege(cmd *shared.Command) *shared.Result {
 var req shared.ManageDatabasePrivilege
 if err:=shared.Unmarshal(cmd.Payload,&req);err!=nil{return errorResult(cmd.Id,"invalid database privilege payload")}
 if err:=managePostgresDatabasePrivilege(req);err!=nil{return errorResult(cmd.Id,err.Error())}
 return &shared.Result{Id:cmd.Id,Ok:true,Message:"database privilege operation completed"}
}
