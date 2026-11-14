package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"veltrix/internal/shared"
)

var slugCleaner = regexp.MustCompile(`[^a-z0-9-]+`)

type projectInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func projectSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "-")
	s = slugCleaner.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 64 { s = strings.Trim(s[:64], "-") }
	return s
}

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	switch r.Method {
	case http.MethodGet:
		rows, err := s.db.conn.QueryContext(r.Context(), `
			SELECT p.id,p.name,p.slug,p.description,p.created_at,p.updated_at,
			(SELECT COUNT(*) FROM project_devices pd WHERE pd.project_id=p.id),
			(SELECT COUNT(*) FROM project_sites ps WHERE ps.project_id=p.id)
			FROM projects p WHERE p.user_id=? ORDER BY p.updated_at DESC`, u.ID)
		if err != nil { apiError(w, shared.ErrInternal("could not load projects")); return }
		defer rows.Close()
		out := make([]map[string]interface{},0)
		for rows.Next() {
			var id int64; var name,slug,description string; var created,updated time.Time; var devices,sites int
			if err:=rows.Scan(&id,&name,&slug,&description,&created,&updated,&devices,&sites);err!=nil { apiError(w,shared.ErrInternal("could not read projects"));return }
			out=append(out,map[string]interface{}{"id":id,"name":name,"slug":slug,"description":description,"created_at":created,"updated_at":updated,"device_count":devices,"site_count":sites})
		}
		if err:=rows.Err();err!=nil { apiError(w,shared.ErrInternal("could not read projects"));return }
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"projects":out})
	case http.MethodPost:
		var in projectInput
		if err:=decodeJSON(r,&in);err!=nil { apiError(w,shared.ErrBadRequest("invalid JSON"));return }
		in.Name=strings.TrimSpace(in.Name)
		if len(in.Name)<2 || len(in.Name)>80 { apiError(w,shared.ErrBadRequest("project name must be 2–80 characters"));return }
		slug:=projectSlug(in.Name); if slug=="" { apiError(w,shared.ErrBadRequest("project name must contain letters or numbers"));return }
		res,err:=s.db.conn.ExecContext(r.Context(),`INSERT INTO projects(user_id,name,slug,description) VALUES(?,?,?,?)`,u.ID,in.Name,slug,strings.TrimSpace(in.Description))
		if err!=nil { apiError(w,shared.ErrBadRequest("project name is already in use"));return }
		id,_:=res.LastInsertId()
		writeJSON(w,http.StatusCreated,map[string]interface{}{"ok":true,"project":map[string]interface{}{"id":id,"name":in.Name,"slug":slug,"description":in.Description}})
	default: http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleProjectByID(w http.ResponseWriter,r *http.Request) {
	u:=UserFromContext(r.Context())
	parts:=strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/projects/"),"/"),"/")
	if len(parts)==0 || parts[0]=="" { http.NotFound(w,r);return }
	id,err:=strconv.ParseInt(parts[0],10,64); if err!=nil || id<1 { apiError(w,shared.ErrBadRequest("invalid project id"));return }
	var name,slug,description string
	err=s.db.conn.QueryRowContext(r.Context(),`SELECT name,slug,description FROM projects WHERE id=? AND user_id=?`,id,u.ID).Scan(&name,&slug,&description)
	if err==sql.ErrNoRows { apiError(w,shared.ErrNotFound("project not found"));return }; if err!=nil { apiError(w,shared.ErrInternal("could not load project"));return }
	if len(parts)==2 && r.Method==http.MethodPost && (parts[1]=="devices" || parts[1]=="sites") {
		var in struct { DeviceID int64 `json:"device_id"`; SiteID string `json:"site_id"` }
		if err:=decodeJSON(r,&in);err!=nil { apiError(w,shared.ErrBadRequest("invalid JSON"));return }
		var count int
		if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM devices WHERE id=? AND user_id=?`,in.DeviceID,u.ID).Scan(&count);err!=nil || count!=1 { apiError(w,shared.ErrNotFound("device not found"));return }
		if parts[1]=="devices" {
			_,err=s.db.conn.ExecContext(r.Context(),`INSERT IGNORE INTO project_devices(project_id,device_id) VALUES(?,?)`,id,in.DeviceID)
		} else {
			if len(in.SiteID)<1 || len(in.SiteID)>64 { apiError(w,shared.ErrBadRequest("invalid site id"));return }
			_,err=s.db.conn.ExecContext(r.Context(),`INSERT IGNORE INTO project_sites(project_id,device_id,site_id) SELECT ?,device_id,site_id FROM sites WHERE device_id=? AND site_id=?`,id,in.DeviceID,in.SiteID)
		}
		if err!=nil { apiError(w,shared.ErrInternal("could not attach resource"));return }
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true});return
	}
	switch r.Method {
	case http.MethodGet:
		rows,err:=s.db.conn.QueryContext(r.Context(),`SELECT d.id,d.hostname,d.device_id FROM project_devices pd JOIN devices d ON d.id=pd.device_id WHERE pd.project_id=? AND d.user_id=?`,id,u.ID)
		if err!=nil { apiError(w,shared.ErrInternal("could not load project devices"));return }
		devices:=make([]map[string]interface{},0); for rows.Next(){var did int64;var host,uuid string;if rows.Scan(&did,&host,&uuid)==nil{devices=append(devices,map[string]interface{}{"id":did,"hostname":host,"device_id":uuid})}};rows.Close()
		srows,err:=s.db.conn.QueryContext(r.Context(),`SELECT ps.device_id,ps.site_id,d.hostname FROM project_sites ps JOIN devices d ON d.id=ps.device_id WHERE ps.project_id=? AND d.user_id=?`,id,u.ID)
		if err!=nil { apiError(w,shared.ErrInternal("could not load project sites"));return }
		sites:=make([]map[string]interface{},0);for srows.Next(){var did int64;var sid,host string;if srows.Scan(&did,&sid,&host)==nil{sites=append(sites,map[string]interface{}{"device_id":did,"site_id":sid,"device_name":host})}};srows.Close()
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"project":map[string]interface{}{"id":id,"name":name,"slug":slug,"description":description},"devices":devices,"sites":sites})
	case http.MethodPatch:
		var in projectInput;if err:=decodeJSON(r,&in);err!=nil{apiError(w,shared.ErrBadRequest("invalid JSON"));return}
		in.Name=strings.TrimSpace(in.Name);if len(in.Name)<2||len(in.Name)>80{apiError(w,shared.ErrBadRequest("project name must be 2–80 characters"));return}
		_,err:=s.db.conn.ExecContext(r.Context(),`UPDATE projects SET name=?,slug=?,description=? WHERE id=? AND user_id=?`,in.Name,projectSlug(in.Name),strings.TrimSpace(in.Description),id,u.ID)
		if err!=nil{apiError(w,shared.ErrBadRequest("project name is already in use"));return};writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true})
	case http.MethodDelete:
		// Удаляется только группировка. Устройства, сайты и их данные остаются нетронутыми.
		_,err:=s.db.conn.ExecContext(r.Context(),`DELETE FROM projects WHERE id=? AND user_id=?`,id,u.ID)
		if err!=nil{apiError(w,shared.ErrInternal("could not delete project"));return};writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true})
	default: http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
	}
}

type databaseServerInput struct {
	DeviceID int64 `json:"device_id"`
	Name string `json:"name"`
	Engine string `json:"engine"`
	Version string `json:"version"`
	Port int `json:"port"`
	DataDirectory string `json:"data_directory"`
	NetworkAccess string `json:"network_access"`
	StorageLimitGB string `json:"storage_limit_gb"`
	RAMBudgetMB int `json:"ram_budget_mb"`
	MaxConnections int `json:"max_connections"`
	MonitorStorage bool `json:"monitor_storage"`
	AutoStart bool `json:"auto_start"`
	ProcessPolicy string `json:"process_policy"`
	BindAddress string `json:"bind_address"`
	SSLMode string `json:"ssl_mode"`
	AuthMethod string `json:"auth_method"`
	RemoteAccess bool `json:"remote_access"`
}

func (s *Server) handleDatabaseServers(w http.ResponseWriter,r *http.Request) {
	u:=UserFromContext(r.Context())
	switch r.Method {
	case http.MethodGet:
		rows,err:=s.db.conn.QueryContext(r.Context(),`SELECT id,device_id,name,engine,version,port,data_directory,network_access,status,created_at FROM database_servers WHERE user_id=? ORDER BY created_at DESC`,u.ID)
		if err!=nil{apiError(w,shared.ErrInternal("could not load database servers"));return}
		defer rows.Close();out:=make([]map[string]interface{},0)
		for rows.Next(){var id,did int64;var name,engine,version,dir,network,status string;var port int;var created time.Time
			if err:=rows.Scan(&id,&did,&name,&engine,&version,&port,&dir,&network,&status,&created);err!=nil{apiError(w,shared.ErrInternal("could not read database servers"));return}
			out=append(out,map[string]interface{}{"id":id,"device_id":did,"name":name,"engine":engine,"version":version,"port":port,"data_directory":dir,"network_access":network,"status":status,"created_at":created})}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"servers":out})
	case http.MethodPost:
		var in databaseServerInput;if err:=decodeJSON(r,&in);err!=nil{apiError(w,shared.ErrBadRequest("invalid JSON"));return}
		in.Name=strings.TrimSpace(in.Name);if in.DeviceID<1||len(in.Name)<2||len(in.Name)>64{apiError(w,shared.ErrBadRequest("device and server name are required"));return}
		if in.Engine!="postgresql"&&in.Engine!="mariadb"&&in.Engine!="mysql"{apiError(w,shared.ErrBadRequest("unsupported database engine"));return}
		if in.Port<1024||in.Port>65535{apiError(w,shared.ErrBadRequest("invalid port"));return}
		if in.NetworkAccess==""{in.NetworkAccess="private"}
		if in.NetworkAccess!="private"&&in.NetworkAccess!="localhost"{apiError(w,shared.ErrBadRequest("network access must be private or localhost"));return}
		var owned int;if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM devices WHERE id=? AND user_id=?`,in.DeviceID,u.ID).Scan(&owned);err!=nil||owned!=1{apiError(w,shared.ErrNotFound("device not found"));return}
		if in.DataDirectory==""{if in.Engine=="postgresql"{in.DataDirectory="$PREFIX/var/lib/postgresql"}else{in.DataDirectory="$PREFIX/var/lib/mysql"}}
		if !strings.HasPrefix(in.DataDirectory,"$PREFIX/")||strings.Contains(in.DataDirectory,".."){apiError(w,shared.ErrBadRequest("data directory must be inside Termux PREFIX"));return}
		if in.StorageLimitGB==""{in.StorageLimitGB="unlimited"}
		if in.StorageLimitGB!="unlimited"{n,e:=strconv.Atoi(in.StorageLimitGB);if e!=nil||n<1||n>100000{apiError(w,shared.ErrBadRequest("storage limit must be unlimited or 1–100000 GB"));return}}
		if in.RAMBudgetMB<0||in.RAMBudgetMB>262144{apiError(w,shared.ErrBadRequest("RAM budget must be between 0 and 262144 MB"));return}
		if in.MaxConnections==0{in.MaxConnections=100};if in.MaxConnections<1||in.MaxConnections>1000{apiError(w,shared.ErrBadRequest("max connections must be 1–1000"));return}
		if in.ProcessPolicy==""{in.ProcessPolicy="balanced"};if in.ProcessPolicy!="balanced"&&in.ProcessPolicy!="persistent"&&in.ProcessPolicy!="manual"{apiError(w,shared.ErrBadRequest("invalid process policy"));return}
		if in.BindAddress==""{in.BindAddress="127.0.0.1"};if in.BindAddress!="127.0.0.1"||in.RemoteAccess{apiError(w,shared.ErrBadRequest("database servers must remain bound to localhost; remote access is disabled"));return}
		if in.SSLMode==""{in.SSLMode="require"};if in.SSLMode!="require"&&in.SSLMode!="prefer"&&in.SSLMode!="disable"{apiError(w,shared.ErrBadRequest("invalid SSL mode"));return}
		if in.AuthMethod==""{in.AuthMethod="password"};if in.AuthMethod!="password"&&in.AuthMethod!="scram-sha-256"{apiError(w,shared.ErrBadRequest("invalid authentication method"));return}
		res,err:=s.db.conn.ExecContext(r.Context(),`INSERT INTO database_servers(user_id,device_id,name,engine,version,port,data_directory,network_access,status,storage_limit_gb,ram_budget_mb,max_connections,monitor_storage,auto_start,process_policy,bind_address,ssl_mode,auth_method,remote_access) VALUES(?,?,?,?,?,?,?,?,'planned',?,?,?,?,?,?,?,?,?,?)`,u.ID,in.DeviceID,in.Name,in.Engine,in.Version,in.Port,in.DataDirectory,in.NetworkAccess,in.StorageLimitGB,in.RAMBudgetMB,in.MaxConnections,in.MonitorStorage,in.AutoStart,in.ProcessPolicy,in.BindAddress,in.SSLMode,in.AuthMethod,in.RemoteAccess)
		if err!=nil{apiError(w,shared.ErrBadRequest("server name or port already in use on this device"));return}
		id,_:=res.LastInsertId()
		writeJSON(w,http.StatusCreated,map[string]interface{}{"ok":true,"server":map[string]interface{}{"id":id,"device_id":in.DeviceID,"name":in.Name,"engine":in.Engine,"version":in.Version,"port":in.Port,"status":"planned","ssl_mode":in.SSLMode,"max_connections":in.MaxConnections},"message":"Configuration saved. Install the PostgreSQL server when the selected device is online."})
	default:http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleInstallDatabaseServer(w http.ResponseWriter,r *http.Request,id int64) {
	u:=UserFromContext(r.Context())
	var deviceID int64;var engine,dataDir,status,bind,sslMode string;var port,maxConnections int;var remote bool
	err:=s.db.conn.QueryRowContext(r.Context(),"SELECT device_id,engine,data_directory,status,port,max_connections,bind_address,remote_access,ssl_mode FROM database_servers WHERE id=? AND user_id=?",id,u.ID).Scan(&deviceID,&engine,&dataDir,&status,&port,&maxConnections,&bind,&remote,&sslMode)
	if err==sql.ErrNoRows{apiError(w,shared.ErrNotFound("database server not found"));return};if err!=nil{apiError(w,shared.ErrInternal("could not load database server"));return}
	if engine!="postgresql"{apiError(w,shared.ErrBadRequest("Agent installation currently supports PostgreSQL only"));return}
	if status=="ready"{apiError(w,shared.ErrBadRequest("database server is already ready"));return}
	if status!="planned"&&status!="error"{apiError(w,shared.ErrBadRequest("database server cannot be installed from its current state"));return}
	if bind!="127.0.0.1"||remote{apiError(w,shared.ErrBadRequest("remote database access must remain disabled"));return}
	conn,online:=s.hub.Get(deviceID);if !online{apiError(w,shared.ErrBadRequest("device is offline"));return}
	res,err:=s.db.conn.ExecContext(r.Context(),"UPDATE database_servers SET status='installing' WHERE id=? AND user_id=? AND status IN ('planned','error')",id,u.ID)
	if err!=nil{apiError(w,shared.ErrInternal("could not queue PostgreSQL installation"));return}
	if n,_:=res.RowsAffected();n!=1{apiError(w,shared.ErrBadRequest("database server state changed; refresh and retry"));return}
	req:=&shared.InstallDatabaseServer{ServerID:id,Engine:engine,Port:uint32(port),DataDirectory:dataDir,MaxConnections:uint32(maxConnections),SSLMode:sslMode}
	go s.runDatabaseServerInstall(id,u.ID,deviceID,conn,req)
	writeJSON(w,http.StatusAccepted,map[string]interface{}{"ok":true,"server_id":id,"status":"installing","message":"PostgreSQL installation queued on the Android device"})
}

func (s *Server) runDatabaseServerInstall(id,userID,deviceID int64,conn *AgentConn,req *shared.InstallDatabaseServer) {
	ctx,cancel:=context.WithTimeout(context.Background(),10*time.Minute);defer cancel()
	result,err:=conn.SendCommand(ctx,"install_database_server",req)
	if err!=nil || !result.Ok {
		_,_=s.db.conn.ExecContext(context.Background(),"UPDATE database_servers SET status='error' WHERE id=? AND user_id=?",id,userID)
		_ = s.db.Log(context.Background(),&shared.LogEntry{UserID:shared.PtrInt64(userID),DeviceID:shared.PtrInt64(deviceID),Action:"database_server_install_failed",Details:map[string]interface{}{"server_id":id}})
		return
	}
	var installed shared.DatabaseServerInstallResult
	if err:=shared.Unmarshal(result.Payload,&installed);err!=nil {
		_,_=s.db.conn.ExecContext(context.Background(),"UPDATE database_servers SET status='error' WHERE id=? AND user_id=?",id,userID)
		return
	}
	if _,err:=s.db.conn.ExecContext(context.Background(),"UPDATE database_servers SET status='ready',version=?,data_directory=? WHERE id=? AND user_id=?",installed.Version,installed.DataDirectory,id,userID);err!=nil {
		_,_=s.db.conn.ExecContext(context.Background(),"UPDATE database_servers SET status='error' WHERE id=? AND user_id=?",id,userID)
		return
	}
	_ = s.db.Log(context.Background(),&shared.LogEntry{UserID:shared.PtrInt64(userID),DeviceID:shared.PtrInt64(deviceID),Action:"database_server_installed",Details:map[string]interface{}{"server_id":id,"engine":req.Engine,"version":installed.Version}})
}

func (s *Server) handleDatabaseServerByID(w http.ResponseWriter,r *http.Request) {
	u:=UserFromContext(r.Context());raw:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/databases/servers/"),"/")
	parts:=strings.Split(raw,"/")
	id,err:=strconv.ParseInt(parts[0],10,64);if err!=nil||id<1{apiError(w,shared.ErrBadRequest("invalid server id"));return}
	if len(parts)==2&&parts[1]=="roles"&&r.Method==http.MethodGet {
		var deviceID int64;var engine,status string;var port int
		err:=s.db.conn.QueryRowContext(r.Context(),`SELECT device_id,engine,status,port FROM database_servers WHERE id=? AND user_id=?`,id,u.ID).Scan(&deviceID,&engine,&status,&port)
		if err==sql.ErrNoRows { apiError(w,shared.ErrNotFound("database server not found"));return }
		if err!=nil { apiError(w,shared.ErrInternal("could not load database server"));return }
		if engine!="postgresql"||status!="ready" { apiError(w,shared.ErrBadRequest("a ready PostgreSQL server is required to inspect roles"));return }
		conn,online:=s.hub.Get(deviceID);if !online { apiError(w,shared.ErrBadRequest("device is offline"));return }
		ctx,cancel:=context.WithTimeout(r.Context(),20*time.Second);defer cancel()
		result,err:=conn.SendCommand(ctx,"list_database_roles",&shared.ListDatabaseRoles{ServerID:id,Port:uint32(port)})
		if err!=nil { apiError(w,shared.ErrInternal("Agent role inventory command failed"));return }
		if !result.Ok { apiError(w,shared.ErrBadRequest(result.Message));return }
		var inventory shared.DatabaseRolesResult
		if err:=shared.Unmarshal(result.Payload,&inventory);err!=nil { apiError(w,shared.ErrInternal("Agent returned an invalid role inventory"));return }
		if inventory.Roles==nil { inventory.Roles=[]shared.DatabaseRole{} }
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"roles":inventory.Roles});return
	}
	if len(parts)==2&&(parts[1]=="roles")&&r.Method==http.MethodPost{s.handleDatabaseServerRoleMutation(w,r,id);return}
	if len(parts)==2&&parts[1]=="privileges"&&r.Method==http.MethodPost{s.handleDatabaseServerPrivileges(w,r,id);return}
	if len(parts)==2&&(parts[1]=="start"||parts[1]=="stop")&&r.Method==http.MethodPost{s.handleDatabaseServerAction(w,r,id,parts[1]);return}
	if len(parts)==2&&parts[1]=="install"&&r.Method==http.MethodPost{s.handleInstallDatabaseServer(w,r,id);return}
	if len(parts)!=1{http.NotFound(w,r);return}
	var did int64;var name,engine,version,dir,network,status,storage,policy,bind,ssl,auth string;var port,ram,maxConnections int;var monitor,autoStart,remote bool;var created time.Time
	err=s.db.conn.QueryRowContext(r.Context(),`SELECT device_id,name,engine,version,port,data_directory,network_access,status,created_at,storage_limit_gb,ram_budget_mb,max_connections,monitor_storage,auto_start,process_policy,bind_address,ssl_mode,auth_method,remote_access FROM database_servers WHERE id=? AND user_id=?`,id,u.ID).Scan(&did,&name,&engine,&version,&port,&dir,&network,&status,&created,&storage,&ram,&maxConnections,&monitor,&autoStart,&policy,&bind,&ssl,&auth,&remote)
	if err==sql.ErrNoRows{apiError(w,shared.ErrNotFound("database server not found"));return};if err!=nil{apiError(w,shared.ErrInternal("could not load database server"));return}
	switch r.Method{
	case http.MethodGet:writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"server":map[string]interface{}{"id":id,"device_id":did,"name":name,"engine":engine,"version":version,"port":port,"data_directory":dir,"network_access":network,"status":status,"created_at":created,"storage_limit_gb":storage,"ram_budget_mb":ram,"max_connections":maxConnections,"monitor_storage":monitor,"auto_start":autoStart,"process_policy":policy,"bind_address":bind,"ssl_mode":ssl,"auth_method":auth,"remote_access":remote}})
	case http.MethodDelete:
		var count int;if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM db_instances WHERE server_id=?`,id).Scan(&count);err!=nil{apiError(w,shared.ErrInternal("could not inspect databases"));return};if count>0{apiError(w,shared.ErrBadRequest("remove or migrate databases before removing this server"));return}
		_,err:=s.db.conn.ExecContext(r.Context(),`DELETE FROM database_servers WHERE id=? AND user_id=? AND status IN ('planned','error','stopped')`,id,u.ID);if err!=nil{apiError(w,shared.ErrInternal("could not remove server metadata"));return};writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true})
	default:http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleDatabaseServerAction(w http.ResponseWriter,r *http.Request,id int64,action string) {
	u:=UserFromContext(r.Context())
	var deviceID int64;var engine,status,dataDir string;var port int
	err:=s.db.conn.QueryRowContext(r.Context(),`SELECT device_id,engine,status,data_directory,port FROM database_servers WHERE id=? AND user_id=?`,id,u.ID).Scan(&deviceID,&engine,&status,&dataDir,&port)
	if err==sql.ErrNoRows { apiError(w,shared.ErrNotFound("database server not found"));return }
	if err!=nil { apiError(w,shared.ErrInternal("could not load database server"));return }
	if engine!="postgresql" { apiError(w,shared.ErrBadRequest("Agent lifecycle actions currently support PostgreSQL only"));return }
	if action=="start"&&status!="stopped"&&status!="ready" { apiError(w,shared.ErrBadRequest("only a stopped PostgreSQL server can be started"));return }
	if action=="stop"&&status=="stopped" { writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"status":"stopped","message":"PostgreSQL is already stopped"});return }
	if action=="stop"&&status!="ready" { apiError(w,shared.ErrBadRequest("only a ready PostgreSQL server can be stopped"));return }
	conn,online:=s.hub.Get(deviceID);if !online { apiError(w,shared.ErrBadRequest("device is offline"));return }
	previous:=status
	if action=="stop" {
		res,err:=s.db.conn.ExecContext(r.Context(),`UPDATE database_servers SET status='stopping' WHERE id=? AND user_id=? AND status='ready'`,id,u.ID)
		if err!=nil { apiError(w,shared.ErrInternal("could not lock database server state"));return }
		if n,_:=res.RowsAffected();n!=1 { apiError(w,shared.ErrBadRequest("database server state changed; refresh and retry"));return }
	} else {
		res,err:=s.db.conn.ExecContext(r.Context(),`UPDATE database_servers SET status='starting' WHERE id=? AND user_id=? AND status=?`,id,u.ID,status)
		if err!=nil { apiError(w,shared.ErrInternal("could not lock database server state"));return }
		if n,_:=res.RowsAffected();n!=1 { apiError(w,shared.ErrBadRequest("database server state changed; refresh and retry"));return }
	}
	ctx,cancel:=context.WithTimeout(r.Context(),110*time.Second);defer cancel()
	result,err:=conn.SendCommand(ctx,action+"_database_server",&shared.DatabaseServerAction{ServerID:id,Port:uint32(port),DataDirectory:dataDir})
	if err!=nil||!result.Ok {
		if previous=="ready"||previous=="stopped" { _,_=s.db.conn.ExecContext(context.Background(),`UPDATE database_servers SET status=? WHERE id=? AND user_id=? AND status IN ('starting','stopping')`,previous,id,u.ID) }
		if err!=nil { apiError(w,shared.ErrInternal("Agent PostgreSQL lifecycle command failed"));return }
		apiError(w,shared.ErrBadRequest(result.Message));return
	}
	target:="ready";if action=="stop" { target="stopped" }
	if _,err:=s.db.conn.ExecContext(r.Context(),`UPDATE database_servers SET status=? WHERE id=? AND user_id=?`,target,id,u.ID);err!=nil { apiError(w,shared.ErrInternal("PostgreSQL action succeeded but server status could not be saved"));return }
	_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),DeviceID:shared.PtrInt64(deviceID),Action:"database_server_"+action,Details:map[string]interface{}{"server_id":id}})
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"status":target,"message":"PostgreSQL server "+action+" completed"})
}

func (s *Server) handleDatabases(w http.ResponseWriter,r *http.Request) {
	u:=UserFromContext(r.Context())
	if r.Method!=http.MethodGet{http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return}
	rows,err:=s.db.conn.QueryContext(r.Context(),`SELECT d.id,d.server_id,d.name,d.status,d.created_at,ds.name,ds.engine,ds.device_id FROM db_instances d JOIN database_servers ds ON ds.id=d.server_id WHERE d.user_id=? ORDER BY d.created_at DESC`,u.ID)
	if err!=nil{apiError(w,shared.ErrInternal("could not load databases"));return};defer rows.Close()
	out:=make([]map[string]interface{},0);for rows.Next(){var id,sid,did int64;var name,status,serverName,engine string;var created time.Time;if err:=rows.Scan(&id,&sid,&name,&status,&created,&serverName,&engine,&did);err!=nil{apiError(w,shared.ErrInternal("could not read databases"));return};out=append(out,map[string]interface{}{"id":id,"server_id":sid,"name":name,"status":status,"created_at":created,"server_name":serverName,"engine":engine,"device_id":did})}
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"databases":out})
}

func (s *Server) handleLogs(w http.ResponseWriter,r *http.Request) {
	u:=UserFromContext(r.Context());limit:=100
	if v:=r.URL.Query().Get("limit");v!=""{if n,err:=strconv.Atoi(v);err==nil&&n>0&&n<=200{limit=n}}
	rows,err:=s.db.conn.QueryContext(r.Context(),`SELECT l.id,l.action,l.details,l.ip,l.created_at,COALESCE(d.hostname,'') FROM logs l LEFT JOIN devices d ON d.id=l.device_id WHERE l.user_id=? ORDER BY l.id DESC LIMIT ?`,u.ID,limit)
	if err!=nil{apiError(w,shared.ErrInternal("could not load logs"));return};defer rows.Close()
	out:=make([]map[string]interface{},0);for rows.Next(){var id int64;var action,ip,host string;var details []byte;var created time.Time;if err:=rows.Scan(&id,&action,&details,&ip,&created,&host);err!=nil{apiError(w,shared.ErrInternal("could not read logs"));return};var parsed interface{};if len(details)>0{_ = json.Unmarshal(details,&parsed)};out=append(out,map[string]interface{}{"id":id,"action":action,"details":parsed,"ip":ip,"created_at":created,"device":host})}
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"logs":out})
}

func (s *Server) handleNotifications(w http.ResponseWriter,r *http.Request) {
	u:=UserFromContext(r.Context())
	if r.Method==http.MethodGet{
		rows,err:=s.db.conn.QueryContext(r.Context(),`SELECT id,type,title,body,read_at,created_at FROM notifications WHERE user_id=? ORDER BY id DESC LIMIT 100`,u.ID)
		if err!=nil{apiError(w,shared.ErrInternal("could not load notifications"));return};defer rows.Close()
		out:=make([]map[string]interface{},0);for rows.Next(){var id int64;var typ,title,body string;var readAt sql.NullTime;var created time.Time;if err:=rows.Scan(&id,&typ,&title,&body,&readAt,&created);err!=nil{apiError(w,shared.ErrInternal("could not read notifications"));return};var read interface{};if readAt.Valid{read=readAt.Time};out=append(out,map[string]interface{}{"id":id,"type":typ,"title":title,"body":body,"read_at":read,"created_at":created})}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"notifications":out});return
	}
	if r.Method==http.MethodPost{
		var in struct{ID int64 `json:"id"`};if err:=decodeJSON(r,&in);err!=nil||in.ID<1{apiError(w,shared.ErrBadRequest("invalid notification id"));return}
		res,err:=s.db.conn.ExecContext(r.Context(),`UPDATE notifications SET read_at=UTC_TIMESTAMP() WHERE id=? AND user_id=?`,in.ID,u.ID);if err!=nil{apiError(w,shared.ErrInternal("could not update notification"));return};n,_:=res.RowsAffected();if n==0{apiError(w,shared.ErrNotFound("notification not found"));return};writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true});return
	}
	http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
}

func (s *Server) handleSettings(w http.ResponseWriter,r *http.Request) {
	u:=UserFromContext(r.Context())
	if r.Method==http.MethodGet{
		rows,err:=s.db.conn.QueryContext(r.Context(),`SELECT setting_key,setting_value FROM user_settings WHERE user_id=?`,u.ID);if err!=nil{apiError(w,shared.ErrInternal("could not load settings"));return};defer rows.Close()
		settings:=map[string]interface{}{};for rows.Next(){var k string;var v []byte;if rows.Scan(&k,&v)==nil{var parsed interface{};if json.Unmarshal(v,&parsed)==nil{settings[k]=parsed}}}
		if _,ok:=settings["mode"];!ok{settings["mode"]="simple"};if _,ok:=settings["locale"];!ok{settings["locale"]="ru"};if _,ok:=settings["theme"];!ok{settings["theme"]="dark"}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"settings":settings});return
	}
	if r.Method==http.MethodPut{
		var in map[string]json.RawMessage;if err:=decodeJSON(r,&in);err!=nil{apiError(w,shared.ErrBadRequest("invalid JSON"));return}
		allowed:=map[string]bool{"mode":true,"locale":true,"theme":true,"notifications":true}
		for key,raw:=range in{if !allowed[key]{apiError(w,shared.ErrBadRequest("unsupported setting: "+key));return}
			if key=="mode"{var v string;if json.Unmarshal(raw,&v)!=nil||(v!="simple"&&v!="advanced"){apiError(w,shared.ErrBadRequest("mode must be simple or advanced"));return}}
			if key=="locale"{var v string;if json.Unmarshal(raw,&v)!=nil||(v!="ru"&&v!="en"){apiError(w,shared.ErrBadRequest("locale must be ru or en"));return}}
			if key=="theme"{var v string;if json.Unmarshal(raw,&v)!=nil||(v!="dark"&&v!="light"){apiError(w,shared.ErrBadRequest("theme must be dark or light"));return}}
			_,err:=s.db.conn.ExecContext(r.Context(),`INSERT INTO user_settings(user_id,setting_key,setting_value) VALUES(?,?,?) ON DUPLICATE KEY UPDATE setting_value=VALUES(setting_value)`,u.ID,key,[]byte(raw));if err!=nil{apiError(w,shared.ErrInternal("could not save settings"));return}
		}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true});return
	}
	http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
}

func (s *Server) handleReports(w http.ResponseWriter,r *http.Request) {
	u:=UserFromContext(r.Context())
	var devices,sites,running,dbServers int
	_ = s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM devices WHERE user_id=?`,u.ID).Scan(&devices)
	_ = s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM sites s JOIN devices d ON d.id=s.device_id WHERE d.user_id=?`,u.ID).Scan(&sites)
	_ = s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM sites s JOIN devices d ON d.id=s.device_id WHERE d.user_id=? AND s.status='running'`,u.ID).Scan(&running)
	_ = s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM database_servers WHERE user_id=?`,u.ID).Scan(&dbServers)
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"report":map[string]interface{}{"generated_at":time.Now().UTC(),"devices":devices,"sites":sites,"running_sites":running,"database_servers":dbServers}})
}

func (s *Server) handleBackups(w http.ResponseWriter,r *http.Request) {
	u:=UserFromContext(r.Context())
	if r.Method==http.MethodPost {
		var in struct {
			DeviceID int64 `json:"device_id"`
			SiteID string `json:"site_id"`
			DatabaseID int64 `json:"database_id"`
		}
		if err:=decodeJSON(r,&in);err!=nil { apiError(w,shared.ErrBadRequest("invalid backup request"));return }
		if in.DatabaseID>0 {
			var deviceID int64;var serverID int64;var database,engine,databaseStatus,serverStatus string;var port int
			err:=s.db.conn.QueryRowContext(r.Context(),`SELECT ds.device_id,ds.id,d.name,ds.engine,d.status,ds.status,ds.port
				FROM db_instances d JOIN database_servers ds ON ds.id=d.server_id
				WHERE d.id=? AND d.user_id=?`,in.DatabaseID,u.ID).Scan(&deviceID,&serverID,&database,&engine,&databaseStatus,&serverStatus,&port)
			if err==sql.ErrNoRows { apiError(w,shared.ErrNotFound("database not found"));return }
			if err!=nil { apiError(w,shared.ErrInternal("could not load database backup target"));return }
			if engine!="postgresql" || databaseStatus!="ready" || serverStatus!="ready" { apiError(w,shared.ErrBadRequest("database and PostgreSQL server must be ready"));return }
			conn,online:=s.hub.Get(deviceID);if !online { apiError(w,shared.ErrBadRequest("device is offline"));return }
			res,err:=s.db.conn.ExecContext(r.Context(),`INSERT INTO backups(user_id,device_id,database_server_id,resource_type,resource_id,file_name,size_bytes,status) VALUES(?,?,?,?,?,?,0,'queued')`,u.ID,deviceID,serverID,"database",fmt.Sprint(in.DatabaseID),fmt.Sprintf("pending-%d.dump",in.DatabaseID))
			if err!=nil { apiError(w,shared.ErrInternal("could not queue database backup"));return }
			backupID,_:=res.LastInsertId()
			go s.runDatabaseBackup(backupID,u.ID,deviceID,conn,&shared.CreateDatabaseBackup{DatabaseID:in.DatabaseID,ServerID:serverID,Database:database,Port:uint32(port)})
			writeJSON(w,http.StatusAccepted,map[string]interface{}{"ok":true,"backup":map[string]interface{}{"id":backupID,"device_id":deviceID,"resource_type":"database","resource_id":fmt.Sprint(in.DatabaseID),"file_name":fmt.Sprintf("pending-%d.dump",in.DatabaseID),"size_bytes":0,"status":"queued"}})
			return
		}
		if in.DeviceID<1||!validBackupSiteID(in.SiteID){apiError(w,shared.ErrBadRequest("invalid device or site"));return}
		var count int
		if err:=s.db.conn.QueryRowContext(r.Context(),"SELECT COUNT(*) FROM sites st JOIN devices d ON d.id=st.device_id WHERE st.device_id=? AND st.site_id=? AND d.user_id=?",in.DeviceID,in.SiteID,u.ID).Scan(&count);err!=nil||count!=1{apiError(w,shared.ErrNotFound("site not found"));return}
		conn,online:=s.hub.Get(in.DeviceID);if !online{apiError(w,shared.ErrBadRequest("device is offline"));return}
		resourceID:=fmt.Sprintf("%d:%s",in.DeviceID,in.SiteID)
		pendingName:=fmt.Sprintf("pending-%s.tar.gz",in.SiteID)
		res,err:=s.db.conn.ExecContext(r.Context(),"INSERT INTO backups(user_id,device_id,resource_type,resource_id,file_name,size_bytes,status) VALUES(?,?,?,?,?,0,'queued')",u.ID,in.DeviceID,"site",resourceID,pendingName)
		if err!=nil{apiError(w,shared.ErrInternal("could not queue website backup"));return}
		id,_:=res.LastInsertId()
		go s.runSiteBackup(id,u.ID,in.DeviceID,conn,&shared.CreateSiteBackup{SiteId:in.SiteID})
		writeJSON(w,http.StatusAccepted,map[string]interface{}{"ok":true,"backup":map[string]interface{}{"id":id,"device_id":in.DeviceID,"resource_type":"site","resource_id":resourceID,"file_name":pendingName,"size_bytes":0,"status":"queued"}});return
	}
	if r.Method!=http.MethodGet{http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return}
	rows,err:=s.db.conn.QueryContext(r.Context(),`SELECT id,resource_type,resource_id,file_name,size_bytes,status,created_at FROM backups WHERE user_id=? ORDER BY id DESC LIMIT 100`,u.ID)
	if err!=nil{apiError(w,shared.ErrInternal("could not load backups"));return};defer rows.Close()
	out:=make([]map[string]interface{},0);for rows.Next(){var id,size int64;var typ,resource,file,status string;var created time.Time;if err:=rows.Scan(&id,&typ,&resource,&file,&size,&status,&created);err!=nil{apiError(w,shared.ErrInternal("could not read backups"));return};out=append(out,map[string]interface{}{"id":id,"resource_type":typ,"resource_id":resource,"file_name":file,"size_bytes":size,"status":status,"created_at":created})}
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"backups":out})
}

func (s *Server) runSiteBackup(backupID,userID,deviceID int64,conn *AgentConn,req *shared.CreateSiteBackup) {
	_,_=s.db.conn.ExecContext(context.Background(),"UPDATE backups SET status='running' WHERE id=? AND user_id=?",backupID,userID)
	ctx,cancel:=context.WithTimeout(context.Background(),2*time.Minute);defer cancel()
	result,err:=conn.SendCommand(ctx,"create_site_backup",req)
	if err!=nil||!result.Ok { _,_=s.db.conn.ExecContext(context.Background(),"UPDATE backups SET status='failed' WHERE id=? AND user_id=?",backupID,userID);return }
	var backup shared.SiteBackupResult
	if err:=shared.Unmarshal(result.Payload,&backup);err!=nil||backup.FileName==""||backup.SizeBytes<1 { _,_=s.db.conn.ExecContext(context.Background(),"UPDATE backups SET status='failed' WHERE id=? AND user_id=?",backupID,userID);return }
	_,_=s.db.conn.ExecContext(context.Background(),"UPDATE backups SET file_name=?,size_bytes=?,status='completed' WHERE id=? AND user_id=?",backup.FileName,backup.SizeBytes,backupID,userID)
	_ = s.db.Log(context.Background(),&shared.LogEntry{UserID:shared.PtrInt64(userID),DeviceID:shared.PtrInt64(deviceID),Action:"site_backup_created",Details:map[string]interface{}{"backup_id":backupID,"site_id":req.SiteId,"size_bytes":backup.SizeBytes}})
}

func (s *Server) runDatabaseBackup(backupID,userID,deviceID int64,conn *AgentConn,req *shared.CreateDatabaseBackup) {
	_,_=s.db.conn.ExecContext(context.Background(),`UPDATE backups SET status='running' WHERE id=? AND user_id=?`,backupID,userID)
	ctx,cancel:=context.WithTimeout(context.Background(),13*time.Minute);defer cancel()
	result,err:=conn.SendCommand(ctx,"create_database_backup",req)
	if err!=nil || !result.Ok {
		_,_=s.db.conn.ExecContext(context.Background(),`UPDATE backups SET status='failed' WHERE id=? AND user_id=?`,backupID,userID)
		return
	}
	var backup shared.DatabaseBackupResult
	if err:=shared.Unmarshal(result.Payload,&backup);err!=nil || backup.FileName=="" || backup.SizeBytes<1 || !backup.Verified {
		_,_=s.db.conn.ExecContext(context.Background(),`UPDATE backups SET status='failed' WHERE id=? AND user_id=?`,backupID,userID)
		return
	}
	_,_=s.db.conn.ExecContext(context.Background(),`UPDATE backups SET file_name=?,size_bytes=?,status='verified' WHERE id=? AND user_id=?`,backup.FileName,backup.SizeBytes,backupID,userID)
	_ = s.db.Log(context.Background(),&shared.LogEntry{UserID:shared.PtrInt64(userID),DeviceID:shared.PtrInt64(deviceID),Action:"database_backup_created",Details:map[string]interface{}{"backup_id":backupID,"database_id":req.DatabaseID,"size_bytes":backup.SizeBytes}})
}

func validBackupSiteID(siteID string) bool {
	if len(siteID)<1||len(siteID)>64{return false}
	for _,r:=range siteID{if !((r>='a'&&r<='z')||(r>='A'&&r<='Z')||(r>='0'&&r<='9')||r=='-'||r=='_'){return false}}
	return true
}

var _ = fmt.Sprintf

func (s *Server) handleDomains(w http.ResponseWriter, r *http.Request) {
	u:=UserFromContext(r.Context())
	if r.Method!=http.MethodGet { http.Error(w,"method not allowed",http.StatusMethodNotAllowed); return }
	rows,err:=s.db.conn.QueryContext(r.Context(),"SELECT st.device_id,st.site_id,st.domain,st.port,st.status,COALESCE(d.hostname,'Android host') FROM sites st JOIN devices d ON d.id=st.device_id WHERE d.user_id=? AND st.domain IS NOT NULL AND st.domain<>'' ORDER BY st.updated_at DESC",u.ID)
	if err!=nil { apiError(w,shared.ErrInternal("could not load domains")); return }
	defer rows.Close()
	out:=make([]map[string]interface{},0)
	for rows.Next(){var deviceID int64;var siteID,domain,status,hostname string;var port int
		if err:=rows.Scan(&deviceID,&siteID,&domain,&port,&status,&hostname);err!=nil{apiError(w,shared.ErrInternal("could not read domains"));return}
		out=append(out,map[string]interface{}{"device_id":deviceID,"site_id":siteID,"domain":domain,"port":port,"site_status":status,"device_name":hostname,"routing_status":"not_verified"})
	}
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"domains":out})
}

func (s *Server) handleDeployments(w http.ResponseWriter,r *http.Request) {
	u:=UserFromContext(r.Context())
	if r.Method!=http.MethodGet { http.Error(w,"method not allowed",http.StatusMethodNotAllowed); return }
	rows,err:=s.db.conn.QueryContext(r.Context(),"SELECT l.id,l.action,l.details,l.created_at,COALESCE(d.hostname,'') FROM logs l LEFT JOIN devices d ON d.id=l.device_id WHERE l.user_id=? AND l.action IN ('create_site','start_site','stop_site','git_pull') ORDER BY l.id DESC LIMIT 100",u.ID)
	if err!=nil { apiError(w,shared.ErrInternal("could not load deployment history")); return }
	defer rows.Close()
	out:=make([]map[string]interface{},0)
	for rows.Next(){var id int64;var action,hostname string;var details []byte;var created time.Time
		if err:=rows.Scan(&id,&action,&details,&created,&hostname);err!=nil{apiError(w,shared.ErrInternal("could not read deployment history"));return}
		var parsed interface{};if len(details)>0{_ = json.Unmarshal(details,&parsed)}
		out=append(out,map[string]interface{}{"id":id,"action":action,"details":parsed,"created_at":created,"device":hostname})
	}
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"deployments":out})
}

func (s *Server) handleMonitoring(w http.ResponseWriter,r *http.Request) {
	u:=UserFromContext(r.Context())
	if r.Method!=http.MethodGet { http.Error(w,"method not allowed",http.StatusMethodNotAllowed); return }
	var totalDevices,onlineDevices,totalSites,runningSites,totalDB int
	_ = s.db.conn.QueryRowContext(r.Context(),"SELECT COUNT(*) FROM devices WHERE user_id=?",u.ID).Scan(&totalDevices)
	_ = s.db.conn.QueryRowContext(r.Context(),"SELECT COUNT(*) FROM devices WHERE user_id=? AND status='online'",u.ID).Scan(&onlineDevices)
	_ = s.db.conn.QueryRowContext(r.Context(),"SELECT COUNT(*) FROM sites st JOIN devices d ON d.id=st.device_id WHERE d.user_id=?",u.ID).Scan(&totalSites)
	_ = s.db.conn.QueryRowContext(r.Context(),"SELECT COUNT(*) FROM sites st JOIN devices d ON d.id=st.device_id WHERE d.user_id=? AND st.status='running'",u.ID).Scan(&runningSites)
	_ = s.db.conn.QueryRowContext(r.Context(),"SELECT COUNT(*) FROM database_servers WHERE user_id=?",u.ID).Scan(&totalDB)
	rows,err:=s.db.conn.QueryContext(r.Context(),"SELECT d.id,COALESCE(d.hostname,'Android host'),d.status,d.agent_version,d.last_seen,(SELECT COUNT(*) FROM sites st WHERE st.device_id=d.id),(SELECT COUNT(*) FROM sites st WHERE st.device_id=d.id AND st.status='running'),m.cpu_percent,m.load_1m,m.memory_total_bytes,m.memory_available_bytes,m.storage_total_bytes,m.storage_available_bytes,m.temperature_celsius,m.captured_at FROM devices d LEFT JOIN device_metrics m ON m.id=(SELECT MAX(mx.id) FROM device_metrics mx WHERE mx.device_id=d.id) WHERE d.user_id=? ORDER BY d.last_seen DESC",u.ID)
	if err!=nil{apiError(w,shared.ErrInternal("could not load monitoring data"));return};defer rows.Close()
	devices:=make([]map[string]interface{},0)
	for rows.Next(){var id int64;var hostname,status,version string;var lastSeen,metricAt sql.NullTime;var sites,running int
		var cpu,load,temp sql.NullFloat64;var memTotal,memAvailable,storageTotal,storageAvailable sql.NullInt64
		if err:=rows.Scan(&id,&hostname,&status,&version,&lastSeen,&sites,&running,&cpu,&load,&memTotal,&memAvailable,&storageTotal,&storageAvailable,&temp,&metricAt);err!=nil{apiError(w,shared.ErrInternal("could not read monitoring data"));return}
		var seen,metricsAt interface{};if lastSeen.Valid{seen=lastSeen.Time};if metricAt.Valid{metricsAt=metricAt.Time}
		metrics:=map[string]interface{}{};if cpu.Valid{metrics["cpu_percent"]=cpu.Float64};if load.Valid{metrics["load_1m"]=load.Float64};if memTotal.Valid{metrics["memory_total_bytes"]=memTotal.Int64};if memAvailable.Valid{metrics["memory_available_bytes"]=memAvailable.Int64};if storageTotal.Valid{metrics["storage_total_bytes"]=storageTotal.Int64};if storageAvailable.Valid{metrics["storage_available_bytes"]=storageAvailable.Int64};if temp.Valid{metrics["temperature_celsius"]=temp.Float64};if metricsAt!=nil{metrics["captured_at"]=metricsAt}
		history, historyErr := s.db.ListDeviceMetricHistory(r.Context(), id, 20)
		if historyErr != nil { history = []DeviceMetricPoint{} }
		devices=append(devices,map[string]interface{}{"id":id,"hostname":hostname,"status":status,"agent_version":version,"last_seen":seen,"site_count":sites,"running_sites":running,"metrics":metrics,"history":history})
	}
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"summary":map[string]int{"devices":totalDevices,"online_devices":onlineDevices,"sites":totalSites,"running_sites":runningSites,"database_servers":totalDB},"devices":devices})
}

type restoreDatabaseBackupInput struct {
	BackupID int64 `json:"backup_id"`
	Name string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password"`
	SiteID string `json:"site_id"`
}

func (s *Server) handleRestoreDatabaseBackup(w http.ResponseWriter,r *http.Request) {
	if r.Method!=http.MethodPost { http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return }
	u:=UserFromContext(r.Context())
	var in restoreDatabaseBackupInput
	if err:=decodeJSON(r,&in);err!=nil { apiError(w,shared.ErrBadRequest("invalid restore request"));return }
	in.Name=strings.TrimSpace(in.Name);in.Username=strings.TrimSpace(in.Username)
	if in.BackupID<1||!postgresIdentifier.MatchString(in.Name)||!postgresIdentifier.MatchString(in.Username)||len(in.Password)<12||len(in.Password)>256||strings.IndexByte(in.Password,0)>=0 {
		apiError(w,shared.ErrBadRequest("provide a valid backup, database name, username and password (12–256 characters)"));return
	}
	var deviceID int64;var backupServerID sql.NullInt64;var fileName,backupStatus,resourceType string
	err:=s.db.conn.QueryRowContext(r.Context(),`SELECT device_id,database_server_id,file_name,status,resource_type FROM backups WHERE id=? AND user_id=?`,in.BackupID,u.ID).Scan(&deviceID,&backupServerID,&fileName,&backupStatus,&resourceType)
	if err==sql.ErrNoRows { apiError(w,shared.ErrNotFound("database backup not found"));return }
	if err!=nil { apiError(w,shared.ErrInternal("could not load database backup"));return }
	if resourceType!="database"||backupStatus!="verified"||!backupServerID.Valid { apiError(w,shared.ErrBadRequest("only verified PostgreSQL database backups can be restored"));return }
	var engine,serverStatus string;var port int
	err=s.db.conn.QueryRowContext(r.Context(),`SELECT engine,status,port FROM database_servers WHERE id=? AND user_id=? AND device_id=?`,backupServerID.Int64,u.ID,deviceID).Scan(&engine,&serverStatus,&port)
	if err==sql.ErrNoRows { apiError(w,shared.ErrNotFound("source Database Server is no longer available"));return }
	if err!=nil { apiError(w,shared.ErrInternal("could not load restore target server"));return }
	if engine!="postgresql"||serverStatus!="ready" { apiError(w,shared.ErrBadRequest("restore requires the original PostgreSQL server to be ready"));return }
	if in.SiteID!="" {
		if !validBackupSiteID(in.SiteID) { apiError(w,shared.ErrBadRequest("invalid website id"));return }
		var count int;if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM sites st JOIN devices d ON d.id=st.device_id WHERE st.device_id=? AND st.site_id=? AND d.user_id=?`,deviceID,in.SiteID,u.ID).Scan(&count);err!=nil||count!=1 { apiError(w,shared.ErrNotFound("website not found on source device"));return }
	}
	var duplicate int
	if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM db_instances WHERE server_id=? AND name=?`,backupServerID.Int64,in.Name).Scan(&duplicate);err!=nil { apiError(w,shared.ErrInternal("could not validate target database name"));return }
	if duplicate>0 { apiError(w,shared.ErrBadRequest("target database name already exists"));return }
	conn,online:=s.hub.Get(deviceID);if !online { apiError(w,shared.ErrBadRequest("device is offline"));return }
	nonce,ciphertext,err:=encryptCredential(s.credentialKey,[]byte(in.Password));if err!=nil { apiError(w,shared.ErrInternal("could not protect target database credentials"));return }
	res,err:=s.db.conn.ExecContext(r.Context(),`INSERT INTO db_instances(user_id,server_id,name,status) VALUES(?,?,?,'planned')`,u.ID,backupServerID.Int64,in.Name)
	if err!=nil { apiError(w,shared.ErrBadRequest("target database name already exists or could not be reserved"));return }
	targetID,_:=res.LastInsertId()
	jobRes,err:=s.db.conn.ExecContext(r.Context(),`INSERT INTO database_restore_jobs(user_id,backup_id,target_database_id,status) VALUES(?,?,?,'queued')`,u.ID,in.BackupID,targetID)
	if err!=nil { _,_=s.db.conn.ExecContext(context.Background(),`DELETE FROM db_instances WHERE id=? AND user_id=?`,targetID,u.ID);apiError(w,shared.ErrInternal("could not queue restore job"));return }
	jobID,_:=jobRes.LastInsertId()
	req:=&shared.RestoreDatabaseBackup{BackupServerID:backupServerID.Int64,BackupFileName:fileName,Database:shared.CreateDatabase{DatabaseID:targetID,ServerID:backupServerID.Int64,Database:in.Name,Username:in.Username,Password:in.Password,Port:uint32(port)}}
	go s.runDatabaseRestore(jobID,in.BackupID,targetID,u.ID,deviceID,conn,req,in.SiteID,nonce,ciphertext)
	writeJSON(w,http.StatusAccepted,map[string]interface{}{"ok":true,"job_id":jobID,"database":map[string]interface{}{"id":targetID,"name":in.Name,"status":"planned"},"status":"queued","message":"Backup will be restored into a new database; the source database and backup remain untouched."})
}

func (s *Server) runDatabaseRestore(jobID,backupID,targetID,userID,deviceID int64,conn *AgentConn,req *shared.RestoreDatabaseBackup,siteID string,nonce,ciphertext []byte) {
	_,_=s.db.conn.ExecContext(context.Background(),`UPDATE database_restore_jobs SET status='running' WHERE id=? AND user_id=?`,jobID,userID)
	ctx,cancel:=context.WithTimeout(context.Background(),13*time.Minute);defer cancel()
	result,err:=conn.SendCommand(ctx,"restore_database_backup",req)
	if err!=nil||!result.Ok {
		_,_=s.db.conn.ExecContext(context.Background(),`UPDATE database_restore_jobs SET status='failed',error_message='Agent restore failed' WHERE id=? AND user_id=?`,jobID,userID)
		_,_=s.db.conn.ExecContext(context.Background(),`UPDATE db_instances SET status='error' WHERE id=? AND user_id=?`,targetID,userID)
		return
	}
	tx,err:=s.db.conn.BeginTx(context.Background(),nil);if err!=nil{_,_=s.db.conn.ExecContext(context.Background(),"UPDATE database_restore_jobs SET status='failed',error_message='metadata transaction failed' WHERE id=?",jobID);_,_=s.db.conn.ExecContext(context.Background(),"UPDATE db_instances SET status='error' WHERE id=?",targetID);return}
	_,err=tx.Exec(`INSERT INTO database_credentials(database_id,username,ciphertext,nonce,key_version) VALUES(?,?,?,?,1)`,targetID,req.Database.Username,ciphertext,nonce)
	if err==nil { _,err=tx.Exec(`UPDATE db_instances SET status='ready' WHERE id=? AND user_id=?`,targetID,userID) }
	if err==nil&&siteID!="" { _,err=tx.Exec(`INSERT INTO site_database_connections(user_id,device_id,site_id,database_id) VALUES(?,?,?,?)`,userID,deviceID,siteID,targetID) }
	if err==nil { _,err=tx.Exec(`UPDATE database_restore_jobs SET status='completed',error_message=NULL WHERE id=? AND user_id=?`,jobID,userID) }
	if err!=nil { _=tx.Rollback();_,_=s.db.conn.ExecContext(context.Background(),`UPDATE database_restore_jobs SET status='failed',error_message='Control Plane metadata save failed' WHERE id=? AND user_id=?`,jobID,userID);_,_=s.db.conn.ExecContext(context.Background(),`UPDATE db_instances SET status='error' WHERE id=? AND user_id=?`,targetID,userID);return }
	if err:=tx.Commit();err!=nil { _,_=s.db.conn.ExecContext(context.Background(),`UPDATE database_restore_jobs SET status='failed',error_message='Control Plane metadata commit failed' WHERE id=? AND user_id=?`,jobID,userID);_,_=s.db.conn.ExecContext(context.Background(),`UPDATE db_instances SET status='error' WHERE id=? AND user_id=?`,targetID,userID);return }
	_ = s.db.Log(context.Background(),&shared.LogEntry{UserID:shared.PtrInt64(userID),DeviceID:shared.PtrInt64(deviceID),Action:"database_backup_restored",Details:map[string]interface{}{"job_id":jobID,"backup_id":backupID,"database_id":targetID}})
}
