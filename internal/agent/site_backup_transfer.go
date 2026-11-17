package agent

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"veltrix/internal/shared"
)

var siteBackupFilePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}-[0-9]{8}T[0-9]{6}\.[0-9]{6}Z\.tar\.gz$`)

func readSiteBackupChunk(req shared.ReadSiteBackupChunk) (*shared.DatabaseBackupChunk,error) {
	if !validSiteID(req.SiteId)||!siteBackupFilePattern.MatchString(req.FileName)||!strings.HasPrefix(req.FileName,req.SiteId+"-"){return nil,fmt.Errorf("invalid site backup file")}
	if req.Offset<0||req.Length<1||req.Length>1<<20{return nil,fmt.Errorf("invalid backup chunk range")}
	prefix:=os.Getenv("PREFIX");if prefix==""{return nil,fmt.Errorf("Termux PREFIX is not set")}
	relative:=filepath.Join("var","lib","veltrix-agent","backups","sites",req.SiteId)
	dir,err:=safeExistingSubdirectory(prefix,relative);if err!=nil{return nil,err}
	path:=filepath.Join(dir,req.FileName)
	info,err:=os.Lstat(path);if err!=nil{return nil,err}
	if !info.Mode().IsRegular()||info.Mode()&os.ModeSymlink!=0{return nil,fmt.Errorf("backup file is not a regular file")}
	if req.Offset>info.Size(){return nil,fmt.Errorf("backup offset exceeds file size")}
	file,err:=os.Open(path);if err!=nil{return nil,err};defer file.Close()
	buf:=make([]byte,req.Length)
	n,readErr:=file.ReadAt(buf,req.Offset)
	if readErr!=nil && readErr!=io.EOF{return nil,readErr}
	buf=buf[:n]
	return &shared.DatabaseBackupChunk{Data:buf,Offset:req.Offset,TotalSize:info.Size(),EOF:req.Offset+int64(n)>=info.Size()},nil
}
