package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"veltrix/internal/shared"
)

var databaseBackupFilePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}-[0-9]+-[0-9]{8}T[0-9]{6}\.[0-9]{6}Z\.dump$`)

func restorePostgresDatabase(req shared.RestoreDatabaseBackup) error {
	if req.BackupServerID<1 || req.Database.ServerID<1 || req.Database.Port<1024 || req.Database.Port>65535 || !databaseBackupFilePattern.MatchString(req.BackupFileName) || filepath.Base(req.BackupFileName)!=req.BackupFileName {
		return fmt.Errorf("invalid database restore request")
	}
	prefix:=os.Getenv("PREFIX");if prefix==""{return fmt.Errorf("Termux PREFIX is not set")}
	backupDir:=filepath.Join(prefix,"var","lib","veltrix-agent","backups","databases",fmt.Sprint(req.BackupServerID))
	fullPath:=filepath.Join(backupDir,req.BackupFileName)
	info,err:=os.Lstat(fullPath);if err!=nil{return fmt.Errorf("database backup file not found")}
	if info.Mode()&os.ModeSymlink!=0||!info.Mode().IsRegular(){return fmt.Errorf("database backup file is not a regular file")}
	realDir,err:=filepath.EvalSymlinks(backupDir);if err!=nil{return fmt.Errorf("could not verify backup directory")}
	realFile,err:=filepath.EvalSymlinks(fullPath);if err!=nil{return fmt.Errorf("could not verify backup file")}
	rel,err:=filepath.Rel(realDir,realFile);if err!=nil||rel==".."||strings.HasPrefix(rel,".."+string(filepath.Separator)){return fmt.Errorf("backup path escapes private storage")}
	if err:=createPostgresDatabase(req.Database);err!=nil{return err}
	pgRestore,err:=exec.LookPath("pg_restore");if err!=nil{cleanupRestoredDatabase(req.Database);return fmt.Errorf("pg_restore is not installed")}
	ctx,cancel:=context.WithTimeout(context.Background(),12*time.Minute);defer cancel()
	cmd:=exec.CommandContext(ctx,pgRestore,"--exit-on-error","--no-owner","--no-acl","-h",filepath.Join(prefix,"tmp"),"-p",fmt.Sprint(req.Database.Port),"-d",req.Database.Database,fullPath)
	out,err:=cmd.CombinedOutput()
	if err!=nil {
		cleanupRestoredDatabase(req.Database)
		if ctx.Err()!=nil{return fmt.Errorf("database restore timed out; new target database was cleaned up")}
		msg:=strings.TrimSpace(string(out));if len(msg)>500{msg=msg[len(msg)-500:]}
		if msg==""{return fmt.Errorf("database restore failed; new target database was cleaned up")}
		return fmt.Errorf("database restore failed; new target database was cleaned up: %s",msg)
	}
	return nil
}

func cleanupRestoredDatabase(req shared.CreateDatabase) {
	prefix:=os.Getenv("PREFIX");if prefix==""{return}
	psql,err:=exec.LookPath("psql");if err!=nil{return}
	sqlText:=fmt.Sprintf("DROP DATABASE IF EXISTS %s;\nDROP ROLE IF EXISTS %s;",req.Database,req.Username)
	ctx,cancel:=context.WithTimeout(context.Background(),30*time.Second);defer cancel()
	_ = runPostgresSQL(ctx,psql,filepath.Join(prefix,"tmp"),req.Port,"postgres",sqlText,req.Password)
}
