package agent

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"veltrix/internal/shared"
)

const maxSiteRestoreBytes int64 = 2 << 30
const maxSiteRestoreFiles = 100000

func (m *ProcessManager) RestoreSiteBackup(req shared.RestoreSiteBackup) error {
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	if !validSiteID(req.SiteId)||!siteBackupFilePattern.MatchString(req.FileName)||!strings.HasPrefix(req.FileName,req.SiteId+"-"){return fmt.Errorf("invalid site backup request")}
	prefix:=os.Getenv("PREFIX");if prefix==""{return fmt.Errorf("Termux PREFIX is not set")}
	backupDir,err:=safeExistingSubdirectory(prefix,filepath.Join("var","lib","veltrix-agent","backups","sites",req.SiteId));if err!=nil{return fmt.Errorf("site backup directory unavailable: %w",err)}
	backupPath:=filepath.Join(backupDir,req.FileName)
	info,err:=os.Lstat(backupPath);if err!=nil{return err};if !info.Mode().IsRegular()||info.Mode()&os.ModeSymlink!=0||info.Size()<1{return fmt.Errorf("backup file is not a valid regular file")}
	sitesRoot,err:=ensureSafeSubdirectory(m.DataRoot,"sites",0755);if err!=nil{return err}
	siteRoot,err:=ensureSafeSubdirectory(m.DataRoot,filepath.Join("sites",req.SiteId),0755);if err!=nil{return err}
	if stopErr:=m.stopSiteLocked(req.SiteId);stopErr!=nil&&!strings.Contains(stopErr.Error(),"is not running"){return fmt.Errorf("stop site before restore: %w",stopErr)}
	stageDir,err:=os.MkdirTemp(sitesRoot,".restore-"+req.SiteId+"-");if err!=nil{return err}
	stageReady:=false
	defer func(){if !stageReady{_ = os.RemoveAll(stageDir)}}()
	if err:=extractSiteArchive(backupPath,stageDir);err!=nil{return err}
	oldPath:=filepath.Join(sitesRoot,".restore-old-"+req.SiteId+"-"+fmt.Sprint(time.Now().UnixNano()))
	if err:=os.Rename(siteRoot,oldPath);err!=nil{return fmt.Errorf("stage current site before restore: %w",err)}
	if err:=os.Rename(stageDir,siteRoot);err!=nil{
		_ = os.Rename(oldPath,siteRoot)
		return fmt.Errorf("activate restored site: %w",err)
	}
	stageReady=true
	if err:=os.RemoveAll(oldPath);err!=nil{return fmt.Errorf("restored site but could not remove previous directory: %w",err)}
	return nil
}

func extractSiteArchive(archivePath,destination string) error {
	file,err:=os.Open(archivePath);if err!=nil{return err};defer file.Close()
	gz,err:=gzip.NewReader(file);if err!=nil{return fmt.Errorf("invalid gzip archive: %w",err)};defer gz.Close()
	tr:=tar.NewReader(gz)
	var total int64;count:=0
	for {
		header,err:=tr.Next()
		if err==io.EOF{break};if err!=nil{return fmt.Errorf("read site archive: %w",err)}
		count++;if count>maxSiteRestoreFiles{return fmt.Errorf("site archive contains too many entries")}
		if header.Size<0||header.Size>maxSiteRestoreBytes-total{return fmt.Errorf("site archive exceeds the 2 GiB extraction limit")}
		name:=header.Name
		if name==""||strings.Contains(name,"\\")||strings.HasPrefix(name,"/"){return fmt.Errorf("archive contains an unsafe path")}
		clean:=filepath.Clean(filepath.FromSlash(name))
		if clean=="."||clean==".."||filepath.IsAbs(clean)||strings.HasPrefix(clean,".."+string(filepath.Separator)){return fmt.Errorf("archive path escapes the site directory")}
		target:=filepath.Join(destination,clean)
		rel,err:=filepath.Rel(destination,target);if err!=nil||rel==".."||strings.HasPrefix(rel,".."+string(filepath.Separator)){return fmt.Errorf("archive path escapes the site directory")}
		switch header.Typeflag {
		case tar.TypeDir:
			if err:=os.MkdirAll(target,0700);err!=nil{return err}
		case tar.TypeReg,tar.TypeRegA:
			if err:=os.MkdirAll(filepath.Dir(target),0700);err!=nil{return err}
			out,err:=os.OpenFile(target,os.O_CREATE|os.O_EXCL|os.O_WRONLY,0600);if err!=nil{return err}
			written,copyErr:=io.CopyN(out,tr,header.Size);closeErr:=out.Close()
			if copyErr!=nil{return copyErr};if closeErr!=nil{return closeErr};if written!=header.Size{return fmt.Errorf("archive file size mismatch")}
			mode:=os.FileMode(header.Mode)&0755;if mode==0{mode=0600};if err:=os.Chmod(target,mode);err!=nil{return err}
			total+=written
		default:
			// Symlinks, hard links, devices, FIFOs and sparse extensions are
			// deliberately rejected to prevent archive path and link attacks.
			return fmt.Errorf("archive contains an unsupported entry type")
		}
	}
	if total==0{return fmt.Errorf("site archive contains no file data")}
	return nil
}
