package agent

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"os"
	"os/user"
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
	prefix:=os.Getenv("PREFIX")
	if prefix=="" { return fmt.Errorf("Termux PREFIX is not set") }
	current,err:=user.Current();if err!=nil || !pgIdentifier.MatchString(current.Username) { return fmt.Errorf("could not safely resolve the local PostgreSQL administrator role") }
	literal:=strings.ReplaceAll(req.Password,"'","''")
	socketDir:=prefix+"/tmp"
	ctx,cancel:=context.WithTimeout(context.Background(),90*time.Second);defer cancel()

	// Create the role and database in separate autocommit commands so failures can be rolled back safely.
	if err:=runPostgresSQL(ctx,psql,socketDir,req.Port,"postgres",fmt.Sprintf("CREATE ROLE %s LOGIN PASSWORD '%s';",req.Username,literal),req.Password);err!=nil{return err}
	if err:=runPostgresSQL(ctx,psql,socketDir,req.Port,"postgres",fmt.Sprintf("CREATE DATABASE %s;",req.Database),req.Password);err!=nil{
		_ = runPostgresSQL(ctx,psql,socketDir,req.Port,"postgres",fmt.Sprintf("DROP ROLE IF EXISTS %s;",req.Username),req.Password)
		return err
	}
	if err:=runPostgresSQL(ctx,psql,socketDir,req.Port,"postgres",fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s;",req.Database,req.Username),req.Password);err!=nil{
		cleanupRestoredDatabase(req)
		return err
	}
	grantSQL:=fmt.Sprintf(`GRANT USAGE ON SCHEMA public TO %s;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO %s;
GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO %s;
ALTER DEFAULT PRIVILEGES FOR ROLE %s IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO %s;
ALTER DEFAULT PRIVILEGES FOR ROLE %s IN SCHEMA public GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO %s;`,
		req.Username,req.Username,req.Username,current.Username,req.Username,current.Username,req.Username)
	if err:=runPostgresSQL(ctx,psql,socketDir,req.Port,req.Database,grantSQL,req.Password);err!=nil{
		cleanupRestoredDatabase(req)
		return fmt.Errorf("database was created but application grants could not be completed; new database and role were cleaned up: %w",err)
	}
	return nil
}

func runPostgresSQL(ctx context.Context, psql, socketDir string, port uint32, database, sqlText, secret string) error {
	cmd:=exec.CommandContext(ctx,psql,"-X","-w","-v","ON_ERROR_STOP=1","-h",socketDir,"-p",fmt.Sprint(port),"-d",database)
	cmd.Stdin=strings.NewReader(sqlText)
	var stderr bytes.Buffer;cmd.Stderr=&stderr
	if err:=cmd.Run();err!=nil {
		msg:=strings.TrimSpace(stderr.String());msg=strings.ReplaceAll(msg,secret,"[redacted]")
		if len(msg)>500{msg=msg[len(msg)-500:]}
		if ctx.Err()!=nil{return fmt.Errorf("PostgreSQL command timed out")}
		if msg==""{return fmt.Errorf("PostgreSQL operation failed")}
		return fmt.Errorf("PostgreSQL operation failed: %s",msg)
	}
	return nil
}
