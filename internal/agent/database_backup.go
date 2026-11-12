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

var backupDatabaseIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}$`)

func createDatabaseBackup(dataRoot string, req shared.CreateDatabaseBackup) (*shared.DatabaseBackupResult,error) {
	if req.DatabaseID<1 || req.ServerID<1 || req.Port<1024 || req.Port>65535 || !backupDatabaseIdentifier.MatchString(req.Database) {
		return nil,fmt.Errorf("invalid database backup request")
	}
	pgDump,err:=exec.LookPath("pg_dump");if err!=nil{return nil,fmt.Errorf("pg_dump is not installed")}
	prefix:=os.Getenv("PREFIX");if prefix==""{return nil,fmt.Errorf("Termux PREFIX is not set")}
	backupDir:=filepath.Join(dataRoot,"backups","databases",fmt.Sprint(req.ServerID))
	if err:=os.MkdirAll(backupDir,0o700);err!=nil{return nil,fmt.Errorf("create database backup directory: %w",err)}
	rootReal,err:=filepath.EvalSymlinks(dataRoot);if err!=nil{return nil,err}
	dirReal,err:=filepath.EvalSymlinks(backupDir);if err!=nil{return nil,err}
	rel,err:=filepath.Rel(rootReal,dirReal);if err!=nil||rel==".."||strings.HasPrefix(rel,".."+string(filepath.Separator)){return nil,fmt.Errorf("database backup path escapes agent data directory")}
	name:=fmt.Sprintf("%s-%d-%s.dump",req.Database,req.DatabaseID,time.Now().UTC().Format("20060102T150405.000000Z"))
	fullPath:=filepath.Join(backupDir,name)
	file,err:=os.OpenFile(fullPath,os.O_CREATE|os.O_EXCL|os.O_WRONLY,0o600);if err!=nil{return nil,fmt.Errorf("create database backup file: %w",err)}
	ok:=false
	defer func(){if !ok{_ = file.Close();_ = os.Remove(fullPath)}}()
	ctx,cancel:=context.WithTimeout(context.Background(),12*time.Minute);defer cancel()
	cmd:=exec.CommandContext(ctx,pgDump,"-Fc","--no-owner","--no-acl","-h",filepath.Join(prefix,"tmp"),"-p",fmt.Sprint(req.Port),"-d",req.Database)
	cmd.Stdout=file
	var stderr strings.Builder;cmd.Stderr=&stderr
	if err:=cmd.Run();err!=nil {
		if ctx.Err()!=nil{return nil,fmt.Errorf("database backup timed out")}
		msg:=strings.TrimSpace(stderr.String());if len(msg)>500{msg=msg[len(msg)-500:]}
		if msg==""{return nil,fmt.Errorf("pg_dump failed")}
		return nil,fmt.Errorf("pg_dump failed: %s",msg)
	}
	if err:=file.Sync();err!=nil{return nil,err}
	if err:=file.Close();err!=nil{return nil,err}
	ok=true
	info,err:=os.Stat(fullPath);if err!=nil{return nil,err}
	return &shared.DatabaseBackupResult{FileName:name,SizeBytes:info.Size(),Format:"custom"},nil
}
