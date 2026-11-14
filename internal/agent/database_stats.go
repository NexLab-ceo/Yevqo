package agent

import (
 "context"
 "encoding/json"
 "fmt"
 "os"
 "os/exec"
 "strings"
 "time"
 "veltrix/internal/shared"
)

func collectPostgresDatabaseStats(req shared.GetDatabaseStats)(shared.DatabaseStats,error){
 if req.ServerID<1||req.Port<1024||req.Port>65535||!postgresDatabaseName.MatchString(req.Database){return shared.DatabaseStats{},fmt.Errorf("invalid database stats request")}
 prefix:=os.Getenv("PREFIX");if prefix==""{return shared.DatabaseStats{},fmt.Errorf("Termux PREFIX is not set")}
 psql,err:=exec.LookPath("psql");if err!=nil{return shared.DatabaseStats{},fmt.Errorf("psql is not installed")}
 query:=`SELECT json_build_object('size_bytes',pg_database_size(current_database()),'active_connections',(SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND state='active'),'max_connections',current_setting('max_connections')::int,'transactions_committed',xact_commit,'transactions_rolled_back',xact_rollback,'deadlocks',deadlocks)::text FROM pg_stat_database WHERE datname=current_database();`
 ctx,cancel:=context.WithTimeout(context.Background(),15*time.Second);defer cancel()
 cmd:=exec.CommandContext(ctx,psql,"-X","-w","-A","-t","-v","ON_ERROR_STOP=1","-h",prefix+"/tmp","-p",fmt.Sprint(req.Port),"-d",req.Database);cmd.Stdin=strings.NewReader(query)
 env:=make([]string,0,len(os.Environ())+1);for _,item:=range os.Environ(){if !strings.HasPrefix(item,"PG"){env=append(env,item)}};cmd.Env=append(env,"PGCONNECT_TIMEOUT=5")
 out,err:=cmd.Output();if err!=nil{if ctx.Err()!=nil{return shared.DatabaseStats{},fmt.Errorf("database stats query timed out")};return shared.DatabaseStats{},fmt.Errorf("could not query PostgreSQL database stats")}
 var stats shared.DatabaseStats;if err:=json.Unmarshal([]byte(strings.TrimSpace(string(out))),&stats);err!=nil{return stats,fmt.Errorf("invalid PostgreSQL stats response")}
 stats.CapturedAt=time.Now().UTC();return stats,nil
}
