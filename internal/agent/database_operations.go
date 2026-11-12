package agent

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"os"
	"regexp"
	"strings"
	"time"

	"veltrix/internal/shared"
)

var pgIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}$`)

func createPostgresDatabase(req shared.CreateDatabase) error {
	if req.DatabaseID<1 || req.ServerID<1 || req.Port<1024 || req.Port>65535 { return fmt.Errorf("invalid database request") }
	if !pgIdentifier.MatchString(req.Database) || !pgIdentifier.MatchString(req.Username) { return fmt.Errorf("invalid PostgreSQL database or role identifier") }
	if len(req.Password)<12 || len(req.Password)>256 || strings.IndexByte(req.Password,0)>=0 { return fmt.Errorf("invalid password length or content") }
	psql,err:=exec.LookPath("psql");if err!=nil{return fmt.Errorf("psql is not installed; install PostgreSQL client tools first")}
	// The password is sent only through stdin, never command-line arguments or logs.
	literal:=strings.ReplaceAll(req.Password,"'","''")
	db,role:=req.Database,req.Username
	sqlText:=fmt.Sprintf(`CREATE ROLE %s LOGIN PASSWORD '%s';
CREATE DATABASE %s OWNER %s;
GRANT CONNECT ON DATABASE %s TO %s;`,role,literal,db,role,db,role)
	ctx,cancel:=context.WithTimeout(context.Background(),90*time.Second);defer cancel()
	prefix:=os.Getenv("PREFIX")
	if prefix=="" { return fmt.Errorf("Termux PREFIX is not set") }
	socketDir:=prefix+"/tmp"
	cmd:=exec.CommandContext(ctx,psql,"-X","-w","-v","ON_ERROR_STOP=1","-h",socketDir,"-p",fmt.Sprint(req.Port),"-U","postgres","-d","postgres")
	cmd.Stdin=strings.NewReader(sqlText)
	var stderr bytes.Buffer;cmd.Stderr=&stderr
	if err:=cmd.Run();err!=nil {
		// Do not expose SQL or password in the returned error.
		msg:=strings.TrimSpace(stderr.String());if len(msg)>500{msg=msg[len(msg)-500:]}
		if ctx.Err()!=nil{return fmt.Errorf("PostgreSQL command timed out")}
		if msg==""{return fmt.Errorf("PostgreSQL database creation failed")}
		return fmt.Errorf("PostgreSQL database creation failed: %s",msg)
	}
	return nil
}
