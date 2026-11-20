package agent

import "veltrix/internal/shared"

func (h *CommandHandler) handleGetDatabaseStats(cmd *shared.Command)*shared.Result{
 var req shared.GetDatabaseStats;if err:=shared.Unmarshal(cmd.Payload,&req);err!=nil{return errorResult(cmd.Id,"invalid database stats payload")}
 var stats shared.DatabaseStats; var err error; if req.Engine == "mariadb" || req.Engine == "mysql" { stats,err=collectMariaDBStats(req) } else { stats,err=collectPostgresDatabaseStats(req) }; if err!=nil{return errorResult(cmd.Id,err.Error())}
 payload,err:=shared.Marshal(stats);if err!=nil{return errorResult(cmd.Id,"could not encode database stats")}
 return &shared.Result{Id:cmd.Id,Ok:true,Message:"database stats collected",Payload:payload}
}
