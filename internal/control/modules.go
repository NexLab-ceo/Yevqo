package control

import (
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
		if in.Port<1||in.Port>65535{apiError(w,shared.ErrBadRequest("invalid port"));return}
		if in.NetworkAccess==""{in.NetworkAccess="private"}
		if in.NetworkAccess!="private"&&in.NetworkAccess!="localhost"{apiError(w,shared.ErrBadRequest("network access must be private or localhost"));return}
		var owned int;if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM devices WHERE id=? AND user_id=?`,in.DeviceID,u.ID).Scan(&owned);err!=nil||owned!=1{apiError(w,shared.ErrNotFound("device not found"));return}
		if in.DataDirectory==""{if in.Engine=="postgresql"{in.DataDirectory="$PREFIX/var/lib/postgresql"}else{in.DataDirectory="$PREFIX/var/lib/mysql"}}
		if !strings.HasPrefix(in.DataDirectory,"$PREFIX/")||strings.Contains(in.DataDirectory,".."){apiError(w,shared.ErrBadRequest("data directory must be inside Termux PREFIX"));return}
		res,err:=s.db.conn.ExecContext(r.Context(),`INSERT INTO database_servers(user_id,device_id,name,engine,version,port,data_directory,network_access,status) VALUES(?,?,?,?,?,?,?,?, 'planned')`,u.ID,in.DeviceID,in.Name,in.Engine,in.Version,in.Port,in.DataDirectory,in.NetworkAccess)
		if err!=nil{apiError(w,shared.ErrBadRequest("server name or port already in use on this device"));return}
		id,_:=res.LastInsertId()
		writeJSON(w,http.StatusCreated,map[string]interface{}{"ok":true,"server":map[string]interface{}{"id":id,"device_id":in.DeviceID,"name":in.Name,"engine":in.Engine,"version":in.Version,"port":in.Port,"status":"planned"},"message":"Configuration saved. Agent installation is not available until the database lifecycle protocol is deployed."})
	default:http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleDatabaseServerByID(w http.ResponseWriter,r *http.Request) {
	u:=UserFromContext(r.Context());raw:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/databases/servers/"),"/")
	id,err:=strconv.ParseInt(raw,10,64);if err!=nil||id<1{apiError(w,shared.ErrBadRequest("invalid server id"));return}
	var did int64;var name,engine,version,dir,network,status string;var port int;var created time.Time
	err=s.db.conn.QueryRowContext(r.Context(),`SELECT device_id,name,engine,version,port,data_directory,network_access,status,created_at FROM database_servers WHERE id=? AND user_id=?`,id,u.ID).Scan(&did,&name,&engine,&version,&port,&dir,&network,&status,&created)
	if err==sql.ErrNoRows{apiError(w,shared.ErrNotFound("database server not found"));return};if err!=nil{apiError(w,shared.ErrInternal("could not load database server"));return}
	switch r.Method{
	case http.MethodGet:writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"server":map[string]interface{}{"id":id,"device_id":did,"name":name,"engine":engine,"version":version,"port":port,"data_directory":dir,"network_access":network,"status":status,"created_at":created}})
	case http.MethodDelete:
		var count int;if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM databases WHERE server_id=?`,id).Scan(&count);err!=nil{apiError(w,shared.ErrInternal("could not inspect databases"));return};if count>0{apiError(w,shared.ErrBadRequest("remove or migrate databases before removing this server"));return}
		_,err:=s.db.conn.ExecContext(r.Context(),`DELETE FROM database_servers WHERE id=? AND user_id=? AND status='planned'`,id,u.ID);if err!=nil{apiError(w,shared.ErrInternal("could not remove server metadata"));return};writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true})
	default:http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleDatabases(w http.ResponseWriter,r *http.Request) {
	u:=UserFromContext(r.Context())
	if r.Method!=http.MethodGet{http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return}
	rows,err:=s.db.conn.QueryContext(r.Context(),`SELECT d.id,d.server_id,d.name,d.status,d.created_at,ds.name,ds.engine,ds.device_id FROM databases d JOIN database_servers ds ON ds.id=d.server_id WHERE d.user_id=? ORDER BY d.created_at DESC`,u.ID)
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
			if key=="theme"{var v string;if json.Unmarshal(raw,&v)!=nil||v!="dark"{apiError(w,shared.ErrBadRequest("unsupported theme"));return}}
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
	rows,err:=s.db.conn.QueryContext(r.Context(),`SELECT id,resource_type,resource_id,file_name,size_bytes,status,created_at FROM backups WHERE user_id=? ORDER BY id DESC LIMIT 100`,u.ID)
	if err!=nil{apiError(w,shared.ErrInternal("could not load backups"));return};defer rows.Close()
	out:=make([]map[string]interface{},0);for rows.Next(){var id,size int64;var typ,resource,file,status string;var created time.Time;if err:=rows.Scan(&id,&typ,&resource,&file,&size,&status,&created);err!=nil{apiError(w,shared.ErrInternal("could not read backups"));return};out=append(out,map[string]interface{}{"id":id,"resource_type":typ,"resource_id":resource,"file_name":file,"size_bytes":size,"status":status,"created_at":created})}
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"backups":out})
}

var _ = fmt.Sprintf
