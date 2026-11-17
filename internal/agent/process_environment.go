package agent

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (m *ProcessManager) siteEnvironmentKey()([]byte,error){
	m.envMu.Lock();defer m.envMu.Unlock()
	dir,err:=ensureSafeSubdirectory(m.DataRoot,".yevqo-agent",0700);if err!=nil{return nil,err}
	path:=filepath.Join(dir,"environment.key")
	if info,err:=os.Lstat(path);err==nil{
		if !info.Mode().IsRegular()||info.Mode()&os.ModeSymlink!=0{return nil,fmt.Errorf("agent environment key is not a regular file")}
		key,err:=os.ReadFile(path);if err!=nil{return nil,err};if len(key)!=32{return nil,fmt.Errorf("agent environment key has invalid length")};return key,nil
	}else if !os.IsNotExist(err){return nil,err}
	key:=make([]byte,32);if _,err:=rand.Read(key);err!=nil{return nil,err}
	file,err:=os.OpenFile(path,os.O_CREATE|os.O_EXCL|os.O_WRONLY,0600);if err!=nil{return nil,err}
	if _,err:=file.Write(key);err!=nil{_ = file.Close();_ = os.Remove(path);return nil,err}
	if err:=file.Sync();err!=nil{_ = file.Close();_ = os.Remove(path);return nil,err}
	if err:=file.Close();err!=nil{return nil,err}
	return key,nil
}

func (m *ProcessManager) siteEnvironmentFile(siteID string)(string,error){
	if !validSiteID(siteID){return "",fmt.Errorf("invalid site id")}
	root,err:=ensureSafeSubdirectory(m.DataRoot,filepath.Join("sites",siteID),0755);if err!=nil{return "",err}
	meta,err:=ensureSafeSubdirectory(root,".yevqo",0700);if err!=nil{return "",err}
	return filepath.Join(meta,"environment.enc"),nil
}

func (m *ProcessManager) saveSiteEnvironment(siteID string,values map[string]string)error{
	path,err:=m.siteEnvironmentFile(siteID);if err!=nil{return err}
	for key,value:=range values{if !validManagedEnvironmentKey(key)||len(value)>16384||strings.IndexByte(value,0)>=0{return fmt.Errorf("invalid managed environment variable")}}
	plain,err:=json.Marshal(values);if err!=nil{return err}
	key,err:=m.siteEnvironmentKey();if err!=nil{return err}
	block,err:=aes.NewCipher(key);if err!=nil{return err};gcm,err:=cipher.NewGCM(block);if err!=nil{return err}
	nonce:=make([]byte,gcm.NonceSize());if _,err:=rand.Read(nonce);err!=nil{return err}
	sealed:=append(nonce,gcm.Seal(nil,nonce,plain,[]byte(siteID))...)
	if info,err:=os.Lstat(path);err==nil&&(!info.Mode().IsRegular()||info.Mode()&os.ModeSymlink!=0){return fmt.Errorf("site environment file is not a regular file")}else if err!=nil&&!os.IsNotExist(err){return err}
	tmp,err:=os.CreateTemp(filepath.Dir(path),".environment-*.tmp");if err!=nil{return err}
	tmpPath:=tmp.Name();ok:=false
	defer func(){if !ok{_ = os.Remove(tmpPath)}}()
	if err:=tmp.Chmod(0600);err!=nil{_ = tmp.Close();return err}
	if _,err:=tmp.Write(sealed);err!=nil{_ = tmp.Close();return err}
	if err:=tmp.Sync();err!=nil{_ = tmp.Close();return err}
	if err:=tmp.Close();err!=nil{return err}
	if err:=os.Rename(tmpPath,path);err!=nil{return err}
	ok=true
	return nil
}

func (m *ProcessManager) loadSiteEnvironment(siteID string)(map[string]string,error){
	path,err:=m.siteEnvironmentFile(siteID);if err!=nil{return nil,err}
	info,err:=os.Lstat(path);if os.IsNotExist(err){return nil,nil};if err!=nil{return nil,err}
	if !info.Mode().IsRegular()||info.Mode()&os.ModeSymlink!=0{return nil,fmt.Errorf("site environment file is not a regular file")}
	sealed,err:=os.ReadFile(path);if err!=nil{return nil,err}
	key,err:=m.siteEnvironmentKey();if err!=nil{return nil,err}
	block,err:=aes.NewCipher(key);if err!=nil{return nil,err};gcm,err:=cipher.NewGCM(block);if err!=nil{return nil,err}
	if len(sealed)<gcm.NonceSize(){return nil,fmt.Errorf("site environment ciphertext is truncated")}
	nonce,ciphertext:=sealed[:gcm.NonceSize()],sealed[gcm.NonceSize():]
	plain,err:=gcm.Open(nil,nonce,ciphertext,[]byte(siteID));if err!=nil{return nil,fmt.Errorf("could not decrypt site environment")}
	var values map[string]string
	if err:=json.Unmarshal(plain,&values);err!=nil{return nil,fmt.Errorf("invalid site environment payload")}
	for key,value:=range values{if !validManagedEnvironmentKey(key)||len(value)>16384||strings.IndexByte(value,0)>=0{return nil,fmt.Errorf("stored site environment contains invalid values")}}
	return values,nil
}
