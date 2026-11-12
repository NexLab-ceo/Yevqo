package agent

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"veltrix/internal/shared"
)

func installDatabaseServer(req shared.InstallDatabaseServer) (*shared.DatabaseServerInstallResult,error) {
	if req.Engine != "postgresql" { return nil, fmt.Errorf("Agent installation currently supports PostgreSQL only; MariaDB/MySQL remain planned") }
	if req.Port < 1024 || req.Port > 65535 { return nil, fmt.Errorf("invalid database port") }
	if req.MaxConnections < 1 || req.MaxConnections > 1000 { return nil, fmt.Errorf("max connections must be 1–1000") }
	prefix:=os.Getenv("PREFIX");if prefix==""{return nil,fmt.Errorf("Termux PREFIX is not set; run the Agent inside Termux")}
	dataDir,err:=resolveTermuxPath(prefix,req.DataDirectory);if err!=nil{return nil,err}
	listener,err:=net.Listen("tcp",fmt.Sprintf("127.0.0.1:%d",req.Port));if err!=nil{return nil,fmt.Errorf("port %d is already in use",req.Port)};_ = listener.Close()
	var fs syscall.Statfs_t;if err:=syscall.Statfs(prefix,&fs);err==nil&&fs.Bavail*uint64(fs.Bsize)<200*1024*1024{return nil,fmt.Errorf("at least 200 MB of free storage is required")}
	ctx,cancel:=context.WithTimeout(context.Background(),8*time.Minute);defer cancel()
	if info,err:=os.Lstat(dataDir);err==nil {
		if info.Mode()&os.ModeSymlink!=0{return nil,fmt.Errorf("database data path must not be a symlink")};if !info.IsDir(){return nil,fmt.Errorf("database data path exists and is not a directory")}
		entries,e:=os.ReadDir(dataDir);if e!=nil{return nil,e}
		if len(entries)>0 { if _,e:=os.Stat(filepath.Join(dataDir,"PG_VERSION"));e==nil{return nil,fmt.Errorf("an existing PostgreSQL cluster was detected; it was not modified. Use an existing-server workflow")};return nil,fmt.Errorf("data directory is not empty; no files were changed") }
	} else if !os.IsNotExist(err) { return nil,err }
	initdb,initErr:=exec.LookPath("initdb")
	if initErr!=nil {
		pkg,err:=exec.LookPath("pkg");if err!=nil{return nil,fmt.Errorf("Termux pkg command was not found")}
		out,err:=exec.CommandContext(ctx,pkg,"install","-y","postgresql").CombinedOutput()
		if err!=nil{return nil,fmt.Errorf("Termux PostgreSQL package install failed: %s",trimOutput(string(out)))}
		initdb,err=exec.LookPath("initdb");if err!=nil{return nil,fmt.Errorf("PostgreSQL package installed but initdb is unavailable")}
	}
	pgctl,err:=exec.LookPath("pg_ctl");if err!=nil{return nil,fmt.Errorf("pg_ctl is unavailable after package installation")}
	if info,err:=os.Lstat(dataDir);err==nil {
		if info.Mode()&os.ModeSymlink!=0{return nil,fmt.Errorf("database data path must not be a symlink")};if !info.IsDir(){return nil,fmt.Errorf("database data path exists and is not a directory")}
		entries,e:=os.ReadDir(dataDir);if e!=nil{return nil,e}
		if len(entries)>0 { if _,e:=os.Stat(filepath.Join(dataDir,"PG_VERSION"));e==nil{return nil,fmt.Errorf("an existing PostgreSQL cluster was detected; it was not modified. Use an existing-server workflow")};return nil,fmt.Errorf("data directory is not empty; no files were changed") }
	} else if !os.IsNotExist(err) { return nil,err }
	if err:=os.MkdirAll(filepath.Dir(dataDir),0o700);err!=nil{return nil,fmt.Errorf("create data parent: %w",err)}
	out,err:=exec.CommandContext(ctx,initdb,"-D",dataDir,"--auth-local=trust","--auth-host=scram-sha-256","--encoding=UTF8","--no-locale").CombinedOutput()
	if err!=nil{return nil,fmt.Errorf("initdb failed: %s",trimOutput(string(out)))}
	conf:=filepath.Join(dataDir,"postgresql.conf")
	config:=map[string]string{"listen_addresses":"'127.0.0.1'","port":fmt.Sprint(req.Port),"max_connections":fmt.Sprint(req.MaxConnections),"password_encryption":"'scram-sha-256'"}
	switch req.SSLMode {
	case "", "require", "prefer":
		cert,key,sslErr:=ensurePostgresSSL(ctx,dataDir)
		if sslErr!=nil{return nil,fmt.Errorf("configure PostgreSQL SSL: %w",sslErr)}
		config["ssl"]="on";config["ssl_cert_file"]="'"+filepath.Base(cert)+"'";config["ssl_key_file"]="'"+filepath.Base(key)+"'"
	case "disable":
		config["ssl"]="off"
	default:
		return nil,fmt.Errorf("unsupported PostgreSQL SSL mode")
	}
	if err:=setPostgresConfig(conf,config);err!=nil{return nil,fmt.Errorf("configure PostgreSQL: %w",err)}
	logDir:=filepath.Join(filepath.Dir(dataDir),"log");if err:=os.MkdirAll(logDir,0o700);err!=nil{return nil,err}
	logFile:=filepath.Join(logDir,fmt.Sprintf("postgresql-%d.log",req.ServerID))
	out,err=exec.CommandContext(ctx,pgctl,"-D",dataDir,"-l",logFile,"-w","-t","120","start").CombinedOutput()
	if err!=nil{return nil,fmt.Errorf("PostgreSQL failed to start: %s",trimOutput(string(out)))}
	if ready,err:=exec.LookPath("pg_isready");err==nil {
		checkCtx,cancelCheck:=context.WithTimeout(ctx,20*time.Second);defer cancelCheck()
		out,err=exec.CommandContext(checkCtx,ready,"-h","127.0.0.1","-p",fmt.Sprint(req.Port)).CombinedOutput()
		if err!=nil{return nil,fmt.Errorf("PostgreSQL started but readiness check failed: %s",trimOutput(string(out)))}
	}
	version:="unknown";if postgres,err:=exec.LookPath("postgres");err==nil{if out,e:=exec.CommandContext(ctx,postgres,"--version").CombinedOutput();e==nil{version=strings.TrimSpace(string(out))}}
	return &shared.DatabaseServerInstallResult{Version:version,DataDirectory:dataDir,Port:req.Port},nil
}

func resolveTermuxPath(prefix,configured string)(string,error){
	if strings.HasPrefix(configured,"$PREFIX/"){configured=filepath.Join(prefix,strings.TrimPrefix(configured,"$PREFIX/"))}
	if !filepath.IsAbs(configured){return "",fmt.Errorf("database path must be absolute or start with $PREFIX/")}
	root,err:=filepath.Abs(prefix);if err!=nil{return "",err};target,err:=filepath.Abs(configured);if err!=nil{return "",err}
	rel,err:=filepath.Rel(root,target);if err!=nil||rel=="."||rel==".."||strings.HasPrefix(rel,".."+string(filepath.Separator)){return "",fmt.Errorf("database path must remain inside Termux PREFIX")}
	rootReal,err:=filepath.EvalSymlinks(root);if err!=nil{return "",err}
	ancestor:=filepath.Dir(target);for { if real,e:=filepath.EvalSymlinks(ancestor);e==nil { realRel,re:=filepath.Rel(rootReal,real);if re!=nil||realRel==".."||strings.HasPrefix(realRel,".."+string(filepath.Separator)){return "",fmt.Errorf("database path resolves outside Termux PREFIX")};break };parent:=filepath.Dir(ancestor);if parent==ancestor{return "",fmt.Errorf("could not resolve database path")};ancestor=parent }
	return target,nil
}

func setPostgresConfig(path string,values map[string]string)error{
	b,err:=os.ReadFile(path);if err!=nil{return err};lines:=strings.Split(string(b),"\n")
	for key,value:=range values{found:=false;for i,line:=range lines{trim:=strings.TrimSpace(line);if strings.HasPrefix(trim,key+" ")||strings.HasPrefix(trim,key+"="){lines[i]=key+" = "+value;found=true;break}};if !found{lines=append(lines,key+" = "+value)}}
	tmp:=path+".veltrix.tmp";if err:=os.WriteFile(tmp,[]byte(strings.Join(lines,"\n")),0o600);err!=nil{return err};return os.Rename(tmp,path)
}

func trimOutput(s string)string{ s=strings.TrimSpace(s);if len(s)>1200{return s[len(s)-1200:]};return s }

func ensurePostgresSSL(ctx context.Context,dataDir string)(string,string,error) {
	cert:=filepath.Join(dataDir,"server.crt")
	key:=filepath.Join(dataDir,"server.key")
	certInfo,certErr:=os.Lstat(cert)
	keyInfo,keyErr:=os.Lstat(key)
	if certErr==nil || keyErr==nil {
		if certErr!=nil || keyErr!=nil || certInfo.Mode()&os.ModeSymlink!=0 || keyInfo.Mode()&os.ModeSymlink!=0 || !certInfo.Mode().IsRegular() || !keyInfo.Mode().IsRegular() {
			return "","",fmt.Errorf("existing SSL certificate paths are unsafe; no files were overwritten")
		}
		if err:=os.Chmod(key,0o600);err!=nil{return "","",err}
		return cert,key,nil
	}
	if !os.IsNotExist(certErr) || !os.IsNotExist(keyErr) { return "","",fmt.Errorf("could not inspect SSL certificate paths") }
	openssl,err:=exec.LookPath("openssl")
	if err!=nil{return "","",fmt.Errorf("openssl is required for SSL mode; install the Termux openssl package")}
	out,err:=exec.CommandContext(ctx,openssl,"req","-new","-x509","-days","3650","-nodes","-out",cert,"-keyout",key,"-subj","/CN=localhost","-addext","subjectAltName=DNS:localhost,IP:127.0.0.1").CombinedOutput()
	if err!=nil{return "","",fmt.Errorf("generate local certificate: %s",trimOutput(string(out)))}
	if err:=os.Chmod(key,0o600);err!=nil{return "","",err}
	if err:=os.Chmod(cert,0o644);err!=nil{return "","",err}
	return cert,key,nil
}
