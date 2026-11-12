package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"veltrix/internal/shared"
)

func testPostgresConnection(req shared.TestDatabaseConnection) error {
	if req.DatabaseID<1 || !pgIdentifier.MatchString(req.Database) || !pgIdentifier.MatchString(req.Username) || req.Port<1024 || req.Port>65535 || len(req.Password)<12 || len(req.Password)>256 {
		return fmt.Errorf("invalid database connection test request")
	}
	if req.SSLMode!="require" && req.SSLMode!="prefer" && req.SSLMode!="disable" { return fmt.Errorf("invalid SSL mode") }
	psql,err:=exec.LookPath("psql");if err!=nil{return fmt.Errorf("psql is not installed")}
	ctx,cancel:=context.WithTimeout(context.Background(),15*time.Second);defer cancel()
	cmd:=exec.CommandContext(ctx,psql,"-X","-w","-v","ON_ERROR_STOP=1","-h","127.0.0.1","-p",fmt.Sprint(req.Port),"-U",req.Username,"-d",req.Database,"-tAc","SELECT 1")
	cmd.Env=append(os.Environ(),"PGPASSWORD="+req.Password,"PGSSLMODE="+req.SSLMode,"PGCONNECT_TIMEOUT=5")
	var stdout,stderr bytes.Buffer;cmd.Stdout=&stdout;cmd.Stderr=&stderr
	if err:=cmd.Run();err!=nil { if ctx.Err()!=nil{return fmt.Errorf("connection test timed out")};return fmt.Errorf("PostgreSQL connection test failed") }
	if strings.TrimSpace(stdout.String())!="1" { return fmt.Errorf("PostgreSQL returned an unexpected test result") }
	return nil
}
