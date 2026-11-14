package agent

import (
 "context"
 "fmt"
 "os"
 "os/exec"
 "regexp"
 "strings"
 "time"
 "veltrix/internal/shared"
)

var postgresDatabaseName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{0,62}$`)

func managePostgresDatabasePrivilege(req shared.ManageDatabasePrivilege) error {
 if req.ServerID<1 || req.Port<1024 || req.Port>65535 || !postgresDatabaseName.MatchString(req.Database) || !postgresRoleName.MatchString(req.Role) || strings.HasPrefix(strings.ToLower(req.Role),"pg_") || req.Role=="postgres" { return fmt.Errorf("invalid database privilege request") }
 if req.Action!="grant" && req.Action!="revoke" { return fmt.Errorf("action must be grant or revoke") }
 scope:=req.Scope;if scope==""{scope="database"}
 if scope=="database" {if req.Privilege!="CONNECT"&&req.Privilege!="CREATE"&&req.Privilege!="TEMPORARY"{return fmt.Errorf("database privilege must be CONNECT, CREATE or TEMPORARY")};if req.Schema!=""{return fmt.Errorf("schema must be empty for database scope")}} else if scope=="schema" {if !postgresDatabaseName.MatchString(req.Schema){return fmt.Errorf("invalid schema name")};if req.Privilege!="USAGE"&&req.Privilege!="CREATE"{return fmt.Errorf("schema privilege must be USAGE or CREATE")}} else{return fmt.Errorf("scope must be database or schema")}
 prefix:=os.Getenv("PREFIX"); if prefix=="" {return fmt.Errorf("Termux PREFIX is not set")}
 psql,err:=exec.LookPath("psql");if err!=nil{return fmt.Errorf("psql is not installed")}
 db:=`"`+strings.ReplaceAll(req.Database,`"`,`""`)+`"`; role:=`"`+strings.ReplaceAll(req.Role,`"`,`""`)+`"`
 query:=req.Action+" "+req.Privilege+" ON DATABASE "+db+" TO "+role
 if scope=="schema" {schema:=`"`+strings.ReplaceAll(req.Schema,`"`,`""`)+`"`;query=req.Action+" "+req.Privilege+" ON SCHEMA "+schema+" TO "+role}
 if req.Action=="revoke" {query+=" CASCADE"}
 ctx,cancel:=context.WithTimeout(context.Background(),20*time.Second);defer cancel()
 cmd:=exec.CommandContext(ctx,psql,"-X","-w","-v","ON_ERROR_STOP=1","-h",prefix+"/tmp","-p",fmt.Sprint(req.Port),"-d","postgres","-c",query)
 env:=make([]string,0,len(os.Environ())+1);for _,item:=range os.Environ(){if !strings.HasPrefix(item,"PG"){env=append(env,item)}};cmd.Env=append(env,"PGCONNECT_TIMEOUT=5")
 var stderr strings.Builder;cmd.Stderr=&stderr
 if err:=cmd.Run();err!=nil {if ctx.Err()!=nil{return fmt.Errorf("database privilege operation timed out")};msg:=strings.TrimSpace(stderr.String());if len(msg)>240{msg=msg[len(msg)-240:]};if msg==""{return fmt.Errorf("database privilege operation failed")};return fmt.Errorf("database privilege operation failed: %s",msg)}
 return nil
}
