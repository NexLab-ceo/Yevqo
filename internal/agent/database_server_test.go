package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"veltrix/internal/shared"
)

func TestResolveTermuxPathAcceptsPrefixPath(t *testing.T) {
	prefix:=t.TempDir()
	got,err:=resolveTermuxPath(prefix,"$PREFIX/var/lib/postgresql")
	if err!=nil{t.Fatal(err)}
	want:=filepath.Join(prefix,"var","lib","postgresql")
	if got!=want{t.Fatalf("got %q, want %q",got,want)}
}

func TestResolveTermuxPathRejectsTraversal(t *testing.T) {
	prefix:=t.TempDir()
	if _,err:=resolveTermuxPath(prefix,"$PREFIX/../../outside");err==nil{t.Fatal("expected traversal to be rejected")}
}

func TestResolveTermuxPathRejectsSymlinkEscape(t *testing.T) {
	prefix:=t.TempDir()
	outside:=t.TempDir()
	if err:=os.Symlink(outside,filepath.Join(prefix,"linked"));err!=nil{t.Skipf("symlinks unavailable: %v",err)}
	if _,err:=resolveTermuxPath(prefix,filepath.Join(prefix,"linked","cluster"));err==nil{t.Fatal("expected symlink escape to be rejected")}
}

func TestSetPostgresConfigReplacesAndAddsValues(t *testing.T) {
	path:=filepath.Join(t.TempDir(),"postgresql.conf")
	if err:=os.WriteFile(path,[]byte("#listen_addresses = 'localhost'\nport = 5432\n"),0o600);err!=nil{t.Fatal(err)}
	if err:=setPostgresConfig(path,map[string]string{"port":"6432","listen_addresses":"'127.0.0.1'"});err!=nil{t.Fatal(err)}
	b,err:=os.ReadFile(path);if err!=nil{t.Fatal(err)}
	s:=string(b)
	if !strings.Contains(s,"port = 6432")||!strings.Contains(s,"listen_addresses = '127.0.0.1'"){t.Fatalf("config not updated: %s",s)}
}

func TestEnsurePostgresSSLRejectsSymlink(t *testing.T) {
	dir:=t.TempDir()
	outside:=filepath.Join(t.TempDir(),"outside.key")
	if err:=os.WriteFile(outside,[]byte("do not overwrite"),0o600);err!=nil{t.Fatal(err)}
	if err:=os.Symlink(outside,filepath.Join(dir,"server.key"));err!=nil{t.Skipf("symlinks unavailable: %v",err)}
	if _,_,err:=ensurePostgresSSL(context.Background(),dir);err==nil{t.Fatal("expected unsafe SSL key symlink to be rejected")}
	got,err:=os.ReadFile(outside);if err!=nil{t.Fatal(err)}
	if string(got)!="do not overwrite"{t.Fatal("outside file was modified")}
}

func TestRestoreDatabaseRejectsBackupDirectorySymlinkEscape(t *testing.T) {
	prefix:=t.TempDir()
	t.Setenv("PREFIX",prefix)
	outside:=t.TempDir()
	name:="source_db-1-20261112T123456.123456Z.dump"
	if err:=os.WriteFile(filepath.Join(outside,name),[]byte("archive"),0o600);err!=nil{t.Fatal(err)}
	link:=filepath.Join(prefix,"var","lib","veltrix-agent","backups","databases","7")
	if err:=os.MkdirAll(filepath.Dir(link),0o700);err!=nil{t.Fatal(err)}
	if err:=os.Symlink(outside,link);err!=nil{t.Skipf("symlinks unavailable: %v",err)}
	req:=shared.RestoreDatabaseBackup{BackupServerID:7,BackupFileName:name,Database:shared.CreateDatabase{DatabaseID:2,ServerID:7,Database:"target_db",Username:"target_user",Password:"long-enough-password",Port:5432}}
	if err:=restorePostgresDatabase(req);err==nil{t.Fatal("expected backup directory symlink escape to be rejected")}
}


func TestManagePostgresServerRejectsInvalidRequest(t *testing.T) {
	if err := managePostgresServer(shared.DatabaseServerAction{}, "start"); err == nil {
		t.Fatal("expected invalid server request to be rejected")
	}
}

func TestManagePostgresServerRejectsUnknownAction(t *testing.T) {
	req := shared.DatabaseServerAction{ServerID: 1, Port: 5432, DataDirectory: "$PREFIX/var/lib/postgresql"}
	if err := managePostgresServer(req, "restart"); err == nil {
		t.Fatal("expected unsupported action to be rejected")
	}
}

func TestRestoreDatabaseRejectsInvalidArchiveBeforeDatabaseMutation(t *testing.T) {
	prefix:=t.TempDir()
	t.Setenv("PREFIX",prefix)
	req:=shared.RestoreDatabaseBackup{BackupServerID:1,BackupFileName:"../../not-a-dump",Database:shared.CreateDatabase{DatabaseID:2,ServerID:1,Database:"target_db",Username:"target_user",Password:"long-enough-password",Port:5432}}
	if err:=restorePostgresDatabase(req);err==nil{t.Fatal("expected invalid backup filename to be rejected")}
	if _,err:=os.Stat(filepath.Join(prefix,"var","lib","veltrix-agent","backups"));!os.IsNotExist(err){t.Fatalf("restore created backup directories before validating request: %v",err)}
}
