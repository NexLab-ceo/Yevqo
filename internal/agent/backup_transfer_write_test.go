package agent

import (
	"os"
	"path/filepath"
	"testing"
	"archive/tar"
	"veltrix/internal/shared"
)

func TestWriteBackupChunkFinalizesVerifiedSiteArchive(t *testing.T) {
	prefix:=t.TempDir();t.Setenv("PREFIX",prefix)
	sourceDir:=t.TempDir();archive:=filepath.Join(sourceDir,"source.tar.gz")
	writeTestSiteArchive(t,archive,[]*tar.Header{{Name:"index.html",Mode:0644,Size:5,Typeflag:tar.TypeReg}},[]string{"hello"})
	data,err:=os.ReadFile(archive);if err!=nil{t.Fatal(err)}
	req:=shared.WriteBackupChunk{TransferID:"0123456789abcdef0123456789abcdef",ResourceType:"site",SiteId:"demo",FileName:"demo-20261117T190000.123456Z.tar.gz",Offset:0,TotalSize:int64(len(data)),Data:data,Final:true}
	result,err:=writeBackupChunk(req);if err!=nil{t.Fatal(err)}
	if !result.Complete||result.Received!=int64(len(data)){t.Fatalf("unexpected transfer result: %#v",result)}
	target:=filepath.Join(prefix,"var","lib","veltrix-agent","backups","sites","demo",req.FileName)
	if _,err:=os.Stat(target);err!=nil{t.Fatalf("final archive missing: %v",err)}
}

func TestWriteBackupChunkRejectsWrongOffsetAndSupportsAbort(t *testing.T) {
	prefix:=t.TempDir();t.Setenv("PREFIX",prefix)
	req:=shared.WriteBackupChunk{TransferID:"abcdef0123456789abcdef0123456789",ResourceType:"site",SiteId:"demo",FileName:"demo-20261117T190000.123456Z.tar.gz",Offset:0,TotalSize:100,Data:[]byte("partial")}
	if _,err:=writeBackupChunk(req);err!=nil{t.Fatal(err)}
	wrong:=req;wrong.Offset=2;wrong.Data=[]byte("x")
	if _,err:=writeBackupChunk(wrong);err==nil{t.Fatal("expected offset mismatch")}
	abort:=shared.WriteBackupChunk{TransferID:req.TransferID,ResourceType:"site",SiteId:req.SiteId,FileName:req.FileName,Abort:true}
	if _,err:=writeBackupChunk(abort);err!=nil{t.Fatal(err)}
	temp:=filepath.Join(prefix,"var","lib","veltrix-agent","backups","sites","demo",".incoming-"+req.TransferID+".part")
	if _,err:=os.Stat(temp);!os.IsNotExist(err){t.Fatalf("temporary transfer file should be removed: %v",err)}
}
