package agent

import (
	"context"
	"os"
	"path/filepath"
	"os"
	"path/filepath"
	"testing"
	"strings"
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
