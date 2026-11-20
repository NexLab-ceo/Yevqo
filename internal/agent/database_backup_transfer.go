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

var safeDatabaseBackupFile = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,180}\.(dump|sql)$`)

func readDatabaseBackupChunk(req shared.ReadDatabaseBackupChunk)(shared.DatabaseBackupChunk,error){
 if req.ServerID<1||!safeDatabaseBackupFile.MatchString(req.FileName)||strings.Contains(req.FileName,"..")||req.Offset<0||req.Length<1||req.Length>256*1024{return shared.DatabaseBackupChunk{},fmt.Errorf("invalid backup chunk request")}
 prefix:=os.Getenv("PREFIX");if prefix==""{return shared.DatabaseBackupChunk{},fmt.Errorf("Termux PREFIX is not set")}
 root:=filepath.Join(prefix,"var","lib","veltrix-agent","backups","databases",fmt.Sprint(req.ServerID))
 prefixReal,err:=filepath.EvalSymlinks(prefix);if err!=nil{return shared.DatabaseBackupChunk{},err}
 rootReal,err:=filepath.EvalSymlinks(root);if err!=nil{return shared.DatabaseBackupChunk{},fmt.Errorf("backup directory unavailable")}
 rootRel,err:=filepath.Rel(prefixReal,rootReal);if err!=nil||rootRel==".."||strings.HasPrefix(rootRel,".."+string(filepath.Separator)){return shared.DatabaseBackupChunk{},fmt.Errorf("backup directory escapes Termux private storage")}
 full:=filepath.Join(root,req.FileName);info,err:=os.Lstat(full);if err!=nil{return shared.DatabaseBackupChunk{},fmt.Errorf("backup file not found")}
 if !info.Mode().IsRegular()||info.Mode()&os.ModeSymlink!=0{return shared.DatabaseBackupChunk{},fmt.Errorf("backup file is not a regular file")}
 real,err:=filepath.EvalSymlinks(full);if err!=nil{return shared.DatabaseBackupChunk{},err}
 rel,err:=filepath.Rel(rootReal,real);if err!=nil||rel==".."||strings.HasPrefix(rel,".."+string(filepath.Separator)){return shared.DatabaseBackupChunk{},fmt.Errorf("backup path escapes private storage")}
 if req.Offset>info.Size(){return shared.DatabaseBackupChunk{},fmt.Errorf("offset exceeds backup size")}
 file,err:=os.Open(real);if err!=nil{return shared.DatabaseBackupChunk{},err};defer file.Close()
 remaining:=info.Size()-req.Offset;n:=req.Length;if int64(n)>remaining{n=int(remaining)}
 data:=make([]byte,n);read,err:=file.ReadAt(data,req.Offset);if err!=nil&&err!=io.EOF{return shared.DatabaseBackupChunk{},err};data=data[:read]
 return shared.DatabaseBackupChunk{Data:data,TotalSize:info.Size(),Offset:req.Offset,EOF:req.Offset+int64(read)>=info.Size()},nil
}
