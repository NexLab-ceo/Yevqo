package agent

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"veltrix/internal/shared"
)

var backupTransferIDPattern=regexp.MustCompile(`^[a-f0-9]{32}$`)

func writeBackupChunk(req shared.WriteBackupChunk)(*shared.WriteBackupChunkResult,error){
	if !backupTransferIDPattern.MatchString(req.TransferID)||req.Offset<0||len(req.Data)>256*1024{return nil,fmt.Errorf("invalid backup transfer chunk")}
	if !req.Abort&&(req.TotalSize<1||req.TotalSize>2<<30||req.Offset+int64(len(req.Data))>req.TotalSize){return nil,fmt.Errorf("invalid backup transfer size")}
	prefix:=os.Getenv("PREFIX");if prefix==""{return nil,fmt.Errorf("Termux PREFIX is not set")}
	var dir,finalName string
	switch req.ResourceType{
	case "site":
		if !validSiteID(req.SiteId)||!siteBackupFilePattern.MatchString(req.FileName)||!strings.HasPrefix(req.FileName,req.SiteId+"-"){return nil,fmt.Errorf("invalid website backup target")}
		var err error
		dir, err = ensureSafeSubdirectory(prefix,filepath.Join("var","lib","veltrix-agent","backups","sites",req.SiteId),0700)
		if err != nil { return nil, err }
		finalName=req.FileName
	case "database":
		if req.ServerID<1||!safeDatabaseBackupFile.MatchString(req.FileName)||strings.Contains(req.FileName,".."){return nil,fmt.Errorf("invalid database backup target")}
		var err error
		dir,err=ensureSafeSubdirectory(prefix,filepath.Join("var","lib","veltrix-agent","backups","databases",fmt.Sprint(req.ServerID)),0700);if err!=nil{return nil,err}
		finalName=req.FileName
	default:return nil,fmt.Errorf("unsupported backup resource type")
	}
	tempPath:=filepath.Join(dir,".incoming-"+req.TransferID+".part")
	finalPath:=filepath.Join(dir,finalName)
	if req.Abort{_ = os.Remove(tempPath);return &shared.WriteBackupChunkResult{},nil}
	if req.Offset==0{
		if _,err:=os.Lstat(finalPath);err==nil{return nil,fmt.Errorf("target backup already exists")}
		file,err:=os.OpenFile(tempPath,os.O_CREATE|os.O_EXCL|os.O_WRONLY,0600);if err!=nil{return nil,err};if len(req.Data)>0{if _,err=file.Write(req.Data);err!=nil{_ = file.Close();_ = os.Remove(tempPath);return nil,err}};if err=file.Sync();err!=nil{_ = file.Close();_ = os.Remove(tempPath);return nil,err};if err=file.Close();err!=nil{return nil,err}
	}else{
		linkInfo,linkErr:=os.Lstat(tempPath);if linkErr!=nil{return nil,linkErr};if !linkInfo.Mode().IsRegular()||linkInfo.Mode()&os.ModeSymlink!=0{return nil,fmt.Errorf("backup transfer temporary file is not regular")}
		file,err:=os.OpenFile(tempPath,os.O_WRONLY,0600);if err!=nil{return nil,err}
		info,err:=file.Stat();if err!=nil{_ = file.Close();return nil,err};if info.Size()!=req.Offset{_ = file.Close();return nil,fmt.Errorf("backup transfer offset mismatch")}
		if len(req.Data)>0{if _,err:=file.WriteAt(req.Data,req.Offset);err!=nil{_ = file.Close();return nil,err}}
		if err=file.Sync();err!=nil{_ = file.Close();return nil,err};if err=file.Close();err!=nil{return nil,err}
	}
	received:=req.Offset+int64(len(req.Data))
	if !req.Final{return &shared.WriteBackupChunkResult{Received:received},nil}
	if received!=req.TotalSize{return nil,fmt.Errorf("final backup transfer size mismatch")}
	info,err:=os.Lstat(tempPath);if err!=nil||!info.Mode().IsRegular()||info.Size()!=req.TotalSize{return nil,fmt.Errorf("transferred backup size verification failed")}
	if req.ResourceType=="site"{if err:=verifySiteArchive(tempPath);err!=nil{return nil,err}}else{pgRestore,err:=exec.LookPath("pg_restore");if err!=nil{return nil,fmt.Errorf("pg_restore is required to verify transferred database backups")};cmd:=exec.Command(pgRestore,"--list",tempPath);if out,err:=cmd.CombinedOutput();err!=nil{return nil,fmt.Errorf("database archive verification failed: %s",strings.TrimSpace(string(out)))}}
	if _,err:=os.Lstat(finalPath);err==nil{return nil,fmt.Errorf("target backup already exists")}
	if err:=os.Rename(tempPath,finalPath);err!=nil{return nil,err}
	return &shared.WriteBackupChunkResult{Received:received,Complete:true},nil
}

func verifySiteArchive(path string)error{
	file,err:=os.Open(path);if err!=nil{return err};defer file.Close()
	gz,err:=gzip.NewReader(file);if err!=nil{return err};defer gz.Close()
	tr:=tar.NewReader(gz);count:=0;var dataBytes int64
	for{h,err:=tr.Next();if err==io.EOF{break};if err!=nil{return err};count++;if count>maxSiteRestoreFiles{return fmt.Errorf("site archive has too many entries")}
		name:=h.Name;if name==""||strings.Contains(name,"\\")||strings.HasPrefix(name,"/"){return fmt.Errorf("unsafe path in site archive")}
		clean:=filepath.Clean(filepath.FromSlash(name));if clean=="."||clean==".."||filepath.IsAbs(clean)||strings.HasPrefix(clean,".."+string(filepath.Separator)){return fmt.Errorf("path traversal in site archive")}
		if h.Typeflag!=tar.TypeDir&&h.Typeflag!=tar.TypeReg&&h.Typeflag!=tar.TypeRegA{return fmt.Errorf("unsupported entry type in site archive")}
		if h.Size<0||h.Size>maxSiteRestoreBytes-dataBytes{return fmt.Errorf("site archive exceeds extraction limit")};if h.Typeflag==tar.TypeReg||h.Typeflag==tar.TypeRegA{dataBytes+=h.Size}
	}
	if count==0||dataBytes==0{return fmt.Errorf("site archive is empty")}
	return nil
}
