package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"veltrix/internal/shared"
)

func TestPruneBackupsKeepsNewestFilesAndHandlesSmallSets(t *testing.T) {
	prefix := t.TempDir()
	t.Setenv("PREFIX", prefix)
	dir := filepath.Join(prefix,"var","lib","veltrix-agent","backups","sites","demo")
	if err:=os.MkdirAll(dir,0700);err!=nil{t.Fatal(err)}
	names:=[]string{"demo-old.tar.gz","demo-middle.tar.gz","demo-new.tar.gz"}
	for i,name:=range names {
		path:=filepath.Join(dir,name)
		if err:=os.WriteFile(path,[]byte(name),0600);err!=nil{t.Fatal(err)}
		stamp:=time.Unix(int64(100+i),0)
		if err:=os.Chtimes(path,stamp,stamp);err!=nil{t.Fatal(err)}
	}
	result,err:=pruneBackups(shared.PruneBackups{SiteId:"demo",Keep:2})
	if err!=nil{t.Fatal(err)}
	if len(result.Removed)!=1||result.Removed[0]!="demo-old.tar.gz"{t.Fatalf("unexpected removed list: %#v",result.Removed)}
	for _,name:=range names[1:] { if _,err:=os.Stat(filepath.Join(dir,name));err!=nil{t.Fatalf("expected %s to remain: %v",name,err)} }
	if _,err:=pruneBackups(shared.PruneBackups{SiteId:"demo",Keep:50});err!=nil{t.Fatalf("keep greater than backup count should be safe: %v",err)}
}

func TestPruneBackupsRejectsSymlinkDirectory(t *testing.T) {
	prefix:=t.TempDir();t.Setenv("PREFIX",prefix)
	outside:=t.TempDir()
	parent:=filepath.Join(prefix,"var","lib","veltrix-agent","backups","sites")
	if err:=os.MkdirAll(parent,0700);err!=nil{t.Fatal(err)}
	if err:=os.Symlink(outside,filepath.Join(parent,"demo"));err!=nil{t.Fatal(err)}
	if _,err:=pruneBackups(shared.PruneBackups{SiteId:"demo",Keep:1});err==nil{t.Fatal("expected symlink backup directory to be rejected")}
}

func TestPruneDatabaseBackupsIsScopedToDatabaseID(t *testing.T) {
	prefix:=t.TempDir();t.Setenv("PREFIX",prefix)
	dir:=filepath.Join(prefix,"var","lib","veltrix-agent","backups","databases","7")
	if err:=os.MkdirAll(dir,0700);err!=nil{t.Fatal(err)}
	files:=[]string{"app-10-20261116T100000.000000Z.dump","app-10-20261117T100000.000000Z.dump","other-20-20261117T110000.000000Z.dump"}
	for i,name:=range files{path:=filepath.Join(dir,name);if err:=os.WriteFile(path,[]byte("dump"),0600);err!=nil{t.Fatal(err)};stamp:=time.Unix(int64(100+i),0);if err:=os.Chtimes(path,stamp,stamp);err!=nil{t.Fatal(err)}}
	result,err:=pruneBackups(shared.PruneBackups{ServerID:7,DatabaseID:10,Keep:1});if err!=nil{t.Fatal(err)}
	if len(result.Removed)!=1||result.Removed[0]!=files[0]{t.Fatalf("unexpected removed database backups: %#v",result.Removed)}
	if _,err:=os.Stat(filepath.Join(dir,files[2]));err!=nil{t.Fatalf("backup for another database must remain: %v",err)}
}
