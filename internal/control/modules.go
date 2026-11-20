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
	OwnerUserID int64 `json:"owner_user_id"`
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
			SELECT p.id,p.user_id,p.name,p.slug,p.description,p.created_at,p.updated_at,
			(SELECT COUNT(*) FROM project_devices pd WHERE pd.project_id=p.id),
			(SELECT COUNT(*) FROM project_sites ps WHERE ps.project_id=p.id)
			FROM projects p WHERE p.user_id IN (SELECT owner_user_id FROM workspace_members WHERE member_user_id=? AND status='active' UNION SELECT ?) ORDER BY p.updated_at DESC`, u.ID, u.ID)
		if err != nil { apiError(w, shared.ErrInternal("could not load projects")); return }
		defer rows.Close()
		out := make([]map[string]interface{},0)
		for rows.Next() {
			var id,ownerID int64; var name,slug,description string; var created,updated time.Time; var devices,sites int
			if err:=rows.Scan(&id,&ownerID,&name,&slug,&description,&created,&updated,&devices,&sites);err!=nil { apiError(w,shared.ErrInternal("could not read projects"));return }
			role,allowed,roleErr:=s.db.WorkspaceRole(r.Context(),ownerID,u.ID);if roleErr!=nil||!allowed{apiError(w,shared.ErrInternal("could not resolve project role"));return}
			out=append(out,map[string]interface{}{"id":id,"owner_user_id":ownerID,"workspace_role":role,"can_manage":CanWorkspaceAction(role,"project.manage"),"name":name,"slug":slug,"description":description,"created_at":created,"updated_at":updated,"device_count":devices,"site_count":sites})
		}
		if err:=rows.Err();err!=nil { apiError(w,shared.ErrInternal("could not read projects"));return }
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"projects":out})
	case http.MethodPost:
		var in projectInput
		if err:=decodeJSON(r,&in);err!=nil { apiError(w,shared.ErrBadRequest("invalid JSON"));return }
		in.Name=strings.TrimSpace(in.Name)
		if len(in.Name)<2 || len(in.Name)>80 { apiError(w,shared.ErrBadRequest("project name must be 2–80 characters"));return }
		slug:=projectSlug(in.Name); if slug=="" { apiError(w,shared.ErrBadRequest("project name must contain letters or numbers"));return }
		ownerID:=in.OwnerUserID;if ownerID==0{ownerID=u.ID}
		if ownerID<1 { apiError(w,shared.ErrBadRequest("invalid workspace owner"));return }
		if err:=s.authorizeWorkspace(r.Context(),ownerID,u.ID,"project.manage");err!=nil{apiError(w,err);return}
		res,err:=s.db.conn.ExecContext(r.Context(),`INSERT INTO projects(user_id,name,slug,description) VALUES(?,?,?,?)`,ownerID,in.Name,slug,strings.TrimSpace(in.Description))
		if err!=nil { apiError(w,shared.ErrBadRequest("project name is already in use"));return }
		id,_:=res.LastInsertId()
		for _, env := range []struct{name,slug,description string; protected bool}{{"Production","production","Live production workloads",true},{"Development","development","Development and integration workloads",false},{"Staging","staging","Pre-production validation workloads",false}} {
			if _,envErr:=s.db.conn.ExecContext(r.Context(),`INSERT INTO project_environments(project_id,name,slug,description,is_protected) VALUES(?,?,?,?,?)`,id,env.name,env.slug,env.description,env.protected);envErr!=nil { _,_=s.db.conn.ExecContext(r.Context(),`DELETE FROM projects WHERE id=? AND user_id=?`,id,ownerID);apiError(w,shared.ErrInternal("could not initialize project environments"));return }
		}
		_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),WorkspaceOwnerID:shared.PtrInt64(ownerID),Action:"project_created",Details:map[string]interface{}{"project_id":id,"name":in.Name},IP:clientIP(r)})
		writeJSON(w,http.StatusCreated,map[string]interface{}{"ok":true,"project":map[string]interface{}{"id":id,"owner_user_id":ownerID,"name":in.Name,"slug":slug,"description":in.Description}})
	default: http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleProjectByID(w http.ResponseWriter,r *http.Request) {
	u:=UserFromContext(r.Context())
	parts:=strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/projects/"),"/"),"/")
	if len(parts)==0 || parts[0]=="" { http.NotFound(w,r);return }
	id,err:=strconv.ParseInt(parts[0],10,64); if err!=nil || id<1 { apiError(w,shared.ErrBadRequest("invalid project id"));return }
	var name,slug,description string
	var ownerID int64
	err=s.db.conn.QueryRowContext(r.Context(),`SELECT user_id,name,slug,description FROM projects WHERE id=?`,id).Scan(&ownerID,&name,&slug,&description)
	if err==sql.ErrNoRows { apiError(w,shared.ErrNotFound("project not found"));return }; if err!=nil { apiError(w,shared.ErrInternal("could not load project"));return }
	action := "project.manage"
	if r.Method == http.MethodGet { action = "project.read" }
	if err := s.authorizeWorkspace(r.Context(), ownerID, u.ID, action); err != nil { apiError(w, err); return }
	if len(parts)==2 && parts[1]=="databases" && r.Method==http.MethodPost {
		var in struct { ServerID int64 `json:"server_id"` }
		if err:=decodeJSON(r,&in);err!=nil||in.ServerID<1 { apiError(w,shared.ErrBadRequest("invalid database server"));return }
		var deviceID int64
		if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT device_id FROM database_servers WHERE id=? AND user_id=?`,in.ServerID,ownerID).Scan(&deviceID);err!=nil { apiError(w,shared.ErrNotFound("database server not found"));return }
		tx,txErr:=s.db.conn.BeginTx(r.Context(),nil)
		if txErr!=nil { apiError(w,shared.ErrInternal("could not start database server attachment"));return }
		if _,err:=tx.ExecContext(r.Context(),`INSERT IGNORE INTO project_devices(project_id,device_id) VALUES(?,?)`,id,deviceID);err==nil {
			_,err=tx.ExecContext(r.Context(),`INSERT IGNORE INTO project_database_servers(project_id,database_server_id) VALUES(?,?)`,id,in.ServerID)
		}
		if err!=nil { _=tx.Rollback();apiError(w,shared.ErrInternal("could not attach database server"));return }
		if err=tx.Commit();err!=nil { apiError(w,shared.ErrInternal("could not commit database server attachment"));return }
		_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),WorkspaceOwnerID:shared.PtrInt64(ownerID),Action:"project_database_attached",Details:map[string]interface{}{"project_id":id,"database_server_id":in.ServerID,"device_id":deviceID},IP:clientIP(r)})
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true});return
	}
	if len(parts)==2 && r.Method==http.MethodDelete && (parts[1]=="devices" || parts[1]=="sites" || parts[1]=="databases") {
		var in struct {
			DeviceID int64 `json:"device_id"`
			SiteID string `json:"site_id"`
			ServerID int64 `json:"server_id"`
		}
		if err:=decodeJSON(r,&in);err!=nil { apiError(w,shared.ErrBadRequest("invalid detach request"));return }
		tx,txErr:=s.db.conn.BeginTx(r.Context(),nil)
		if txErr!=nil { apiError(w,shared.ErrInternal("could not start resource detach transaction"));return }
		switch parts[1] {
		case "sites":
			if in.DeviceID<1||len(in.SiteID)<1||len(in.SiteID)>64 { _=tx.Rollback();apiError(w,shared.ErrBadRequest("device_id and site_id are required"));return }
			if _,err:=tx.ExecContext(r.Context(),`DELETE FROM project_environment_sites WHERE project_id=? AND device_id=? AND site_id=?`,id,in.DeviceID,in.SiteID);err!=nil{_=tx.Rollback();apiError(w,shared.ErrInternal("could not remove environment site assignments"));return}
			res,err:=tx.ExecContext(r.Context(),`DELETE FROM project_sites WHERE project_id=? AND device_id=? AND site_id=?`,id,in.DeviceID,in.SiteID)
			if err!=nil { _=tx.Rollback();apiError(w,shared.ErrInternal("could not detach site"));return }
			n,_:=res.RowsAffected();if n==0 { _=tx.Rollback();apiError(w,shared.ErrNotFound("project site attachment not found"));return }
			var remaining int
			if refErr:=tx.QueryRowContext(r.Context(),`SELECT (SELECT COUNT(*) FROM project_sites WHERE project_id=? AND device_id=?)+(SELECT COUNT(*) FROM project_database_servers pds JOIN database_servers ds ON ds.id=pds.database_server_id WHERE pds.project_id=? AND ds.device_id=?)`,id,in.DeviceID,id,in.DeviceID).Scan(&remaining);refErr!=nil {
				_=tx.Rollback();apiError(w,shared.ErrInternal("could not inspect project device references"));return
			}
			if remaining==0 { if _,err=tx.ExecContext(r.Context(),`DELETE FROM project_devices WHERE project_id=? AND device_id=?`,id,in.DeviceID);err!=nil { _=tx.Rollback();apiError(w,shared.ErrInternal("could not update project device attachment"));return } }
		case "databases":
			if in.ServerID<1 { _=tx.Rollback();apiError(w,shared.ErrBadRequest("server_id is required"));return }
			var deviceID int64
			if err:=tx.QueryRowContext(r.Context(),`SELECT device_id FROM database_servers WHERE id=? AND user_id=?`,in.ServerID,ownerID).Scan(&deviceID);err!=nil { _=tx.Rollback();apiError(w,shared.ErrNotFound("database server not found"));return }
			if _,err:=tx.ExecContext(r.Context(),`DELETE FROM project_environment_databases WHERE project_id=? AND database_id IN (SELECT id FROM db_instances WHERE server_id=?)`,id,in.ServerID);err!=nil{_=tx.Rollback();apiError(w,shared.ErrInternal("could not remove environment database instances"));return}
			if _,err:=tx.ExecContext(r.Context(),`DELETE FROM project_environment_database_servers WHERE project_id=? AND database_server_id=?`,id,in.ServerID);err!=nil{_=tx.Rollback();apiError(w,shared.ErrInternal("could not remove environment database assignments"));return}
			res,err:=tx.ExecContext(r.Context(),`DELETE FROM project_database_servers WHERE project_id=? AND database_server_id=?`,id,in.ServerID)
			if err!=nil { _=tx.Rollback();apiError(w,shared.ErrInternal("could not detach database server"));return }
			n,_:=res.RowsAffected();if n==0 { _=tx.Rollback();apiError(w,shared.ErrNotFound("project database attachment not found"));return }
			var remaining int
			if err:=tx.QueryRowContext(r.Context(),`SELECT (SELECT COUNT(*) FROM project_sites WHERE project_id=? AND device_id=?)+(SELECT COUNT(*) FROM project_database_servers pds JOIN database_servers ds ON ds.id=pds.database_server_id WHERE pds.project_id=? AND ds.device_id=?)`,id,deviceID,id,deviceID).Scan(&remaining);err!=nil { _=tx.Rollback();apiError(w,shared.ErrInternal("could not inspect project device references"));return }
			if remaining==0 { _,err=tx.ExecContext(r.Context(),`DELETE FROM project_devices WHERE project_id=? AND device_id=?`,id,deviceID) }
			if err!=nil { _=tx.Rollback();apiError(w,shared.ErrInternal("could not update project device attachment"));return }
		case "devices":
			if in.DeviceID<1 { _=tx.Rollback();apiError(w,shared.ErrBadRequest("device_id is required"));return }
			var count int
			if err:=tx.QueryRowContext(r.Context(),`SELECT (SELECT COUNT(*) FROM project_sites WHERE project_id=? AND device_id=?)+(SELECT COUNT(*) FROM project_database_servers pds JOIN database_servers ds ON ds.id=pds.database_server_id WHERE pds.project_id=? AND ds.device_id=?)`,id,in.DeviceID,id,in.DeviceID).Scan(&count);err!=nil { _=tx.Rollback();apiError(w,shared.ErrInternal("could not inspect project device references"));return }
			if count>0 { _=tx.Rollback();apiError(w,shared.ErrBadRequest("detach the device's applications and database servers first"));return }
			res,err:=tx.ExecContext(r.Context(),`DELETE FROM project_devices WHERE project_id=? AND device_id=?`,id,in.DeviceID)
			if err!=nil { _=tx.Rollback();apiError(w,shared.ErrInternal("could not detach device"));return }
			n,_:=res.RowsAffected();if n==0 { _=tx.Rollback();apiError(w,shared.ErrNotFound("project device attachment not found"));return }
		}
		if err:=tx.Commit();err!=nil { apiError(w,shared.ErrInternal("could not commit resource detach"));return }
		_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),WorkspaceOwnerID:shared.PtrInt64(ownerID),Action:"project_resource_detached",Details:map[string]interface{}{"project_id":id,"resource_type":parts[1],"device_id":in.DeviceID,"site_id":in.SiteID,"database_server_id":in.ServerID},IP:clientIP(r)})
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true});return
	}
	if len(parts)==2 && r.Method==http.MethodPost && (parts[1]=="devices" || parts[1]=="sites") {
		var in struct { DeviceID int64 `json:"device_id"`; SiteID string `json:"site_id"` }
		if err:=decodeJSON(r,&in);err!=nil { apiError(w,shared.ErrBadRequest("invalid JSON"));return }
		var count int
		if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM devices WHERE id=? AND user_id=?`,in.DeviceID,ownerID).Scan(&count);err!=nil || count!=1 { apiError(w,shared.ErrNotFound("device not found"));return }
		if parts[1]=="devices" {
			_,err=s.db.conn.ExecContext(r.Context(),`INSERT IGNORE INTO project_devices(project_id,device_id) VALUES(?,?)`,id,in.DeviceID)
		} else {
			if len(in.SiteID)<1 || len(in.SiteID)>64 { apiError(w,shared.ErrBadRequest("invalid site id"));return }
			var siteCount int
			if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM sites WHERE device_id=? AND site_id=?`,in.DeviceID,in.SiteID).Scan(&siteCount);err!=nil||siteCount!=1 { apiError(w,shared.ErrNotFound("site not found on selected device"));return }
			tx,txErr:=s.db.conn.BeginTx(r.Context(),nil)
			if txErr!=nil { apiError(w,shared.ErrInternal("could not start project resource transaction"));return }
			if _,err=tx.ExecContext(r.Context(),`INSERT IGNORE INTO project_devices(project_id,device_id) VALUES(?,?)`,id,in.DeviceID);err==nil {
				_,err=tx.ExecContext(r.Context(),`INSERT IGNORE INTO project_sites(project_id,device_id,site_id) VALUES(?,?,?)`,id,in.DeviceID,in.SiteID)
			}
			if err!=nil { _=tx.Rollback();apiError(w,shared.ErrInternal("could not attach resource"));return }
			if err=tx.Commit();err!=nil { apiError(w,shared.ErrInternal("could not commit project resource attachment"));return }
		}
		if err!=nil { apiError(w,shared.ErrInternal("could not attach resource"));return }
		_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),WorkspaceOwnerID:shared.PtrInt64(ownerID),Action:"project_resource_attached",Details:map[string]interface{}{"project_id":id,"resource_type":parts[1],"device_id":in.DeviceID,"site_id":in.SiteID},IP:clientIP(r)})
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true});return
	}
	switch r.Method {
	case http.MethodGet:
		rows,err:=s.db.conn.QueryContext(r.Context(),`SELECT d.id,d.hostname,d.device_id FROM project_devices pd JOIN devices d ON d.id=pd.device_id WHERE pd.project_id=? AND d.user_id=?`,id,ownerID)
		if err!=nil { apiError(w,shared.ErrInternal("could not load project devices"));return }
		devices:=make([]map[string]interface{},0); for rows.Next(){var did int64;var host,uuid string;if rows.Scan(&did,&host,&uuid)==nil{devices=append(devices,map[string]interface{}{"id":did,"hostname":host,"device_id":uuid})}};rows.Close()
		srows,err:=s.db.conn.QueryContext(r.Context(),`SELECT ps.device_id,ps.site_id,d.hostname FROM project_sites ps JOIN devices d ON d.id=ps.device_id WHERE ps.project_id=? AND d.user_id=?`,id,ownerID)
		if err!=nil { apiError(w,shared.ErrInternal("could not load project sites"));return }
		sites:=make([]map[string]interface{},0);for srows.Next(){var did int64;var sid,host string;if srows.Scan(&did,&sid,&host)==nil{sites=append(sites,map[string]interface{}{"device_id":did,"site_id":sid,"device_name":host})}};srows.Close()
		dbRows,err:=s.db.conn.QueryContext(r.Context(),`SELECT ds.id,ds.device_id,ds.name,ds.engine,ds.status FROM project_database_servers pds JOIN database_servers ds ON ds.id=pds.database_server_id WHERE pds.project_id=? AND ds.user_id=? ORDER BY ds.created_at DESC`,id,ownerID)
		if err!=nil { apiError(w,shared.ErrInternal("could not load project database servers"));return }
		databaseServers:=make([]map[string]interface{},0)
		for dbRows.Next(){var serverID,deviceID int64;var serverName,engine,status string;if err:=dbRows.Scan(&serverID,&deviceID,&serverName,&engine,&status);err!=nil{dbRows.Close();apiError(w,shared.ErrInternal("could not read project database servers"));return};databaseServers=append(databaseServers,map[string]interface{}{"id":serverID,"device_id":deviceID,"name":serverName,"engine":engine,"status":status})}
		if err:=dbRows.Err();err!=nil{dbRows.Close();apiError(w,shared.ErrInternal("could not read project database servers"));return};dbRows.Close()
		role,allowed,roleErr:=s.db.WorkspaceRole(r.Context(),ownerID,u.ID);if roleErr!=nil||!allowed{apiError(w,shared.ErrInternal("could not resolve project role"));return}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"project":map[string]interface{}{"id":id,"owner_user_id":ownerID,"workspace_role":role,"can_manage":CanWorkspaceAction(role,"project.manage"),"name":name,"slug":slug,"description":description},"devices":devices,"sites":sites,"database_servers":databaseServers})
	case http.MethodPatch:
		var in projectInput;if err:=decodeJSON(r,&in);err!=nil{apiError(w,shared.ErrBadRequest("invalid JSON"));return}
		in.Name=strings.TrimSpace(in.Name);if len(in.Name)<2||len(in.Name)>80{apiError(w,shared.ErrBadRequest("project name must be 2–80 characters"));return}
		_,err:=s.db.conn.ExecContext(r.Context(),`UPDATE projects SET name=?,slug=?,description=? WHERE id=? AND user_id=?`,in.Name,projectSlug(in.Name),strings.TrimSpace(in.Description),id,ownerID)
		if err!=nil{apiError(w,shared.ErrBadRequest("project name is already in use"));return};_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),WorkspaceOwnerID:shared.PtrInt64(ownerID),Action:"project_updated",Details:map[string]interface{}{"project_id":id,"name":in.Name},IP:clientIP(r)});writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true})
	case http.MethodDelete:
		// Удаляется только группировка. Устройства, сайты и их данные остаются нетронутыми.
		_,err:=s.db.conn.ExecContext(r.Context(),`DELETE FROM projects WHERE id=? AND user_id=?`,id,ownerID)
		if err!=nil{apiError(w,shared.ErrInternal("could not delete project"));return};_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),WorkspaceOwnerID:shared.PtrInt64(ownerID),Action:"project_deleted",Details:map[string]interface{}{"project_id":id,"name":name},IP:clientIP(r)});writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true})
	default: http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
	}
}

type databaseServerInput struct {
	DeviceID int64 `json:"device_id"`
	ProjectID int64 `json:"project_id"`
	EnvironmentID int64 `json:"environment_id"`
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
		projectID := int64(0)
		environmentID := int64(0)
		var owners []int64
		if raw := r.URL.Query().Get("project_id"); raw != "" {
			parsed, parseErr := strconv.ParseInt(raw, 10, 64)
			if parseErr != nil || parsed < 1 { apiError(w, shared.ErrBadRequest("invalid project id")); return }
			ownerID, accessErr := s.authorizeProjectOwner(r.Context(), parsed, u.ID, "database.read")
			if accessErr != nil { apiError(w, accessErr); return }
			projectID = parsed
			owners = []int64{ownerID}
		} else {
			allOwners, ownerErr := s.db.WorkspaceOwnerIDs(r.Context(), u.ID)
			if ownerErr != nil { apiError(w, ownerErr); return }
			for _, ownerID := range allOwners {
				role, allowed, roleErr := s.db.WorkspaceRole(r.Context(), ownerID, u.ID)
				if roleErr != nil { apiError(w, roleErr); return }
				if allowed && CanWorkspaceAction(role, "database.read") { owners = append(owners, ownerID) }
			}
		}
		if raw := r.URL.Query().Get("environment_id"); raw != "" {
			parsed, parseErr := strconv.ParseInt(raw, 10, 64)
			if parseErr != nil || parsed < 1 || projectID < 1 { apiError(w, shared.ErrBadRequest("environment_id requires a valid project_id")); return }
			var exists int
			if err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM project_environments WHERE id=? AND project_id=?`, parsed, projectID).Scan(&exists); err != nil || exists != 1 { apiError(w, shared.ErrNotFound("environment not found")); return }
			environmentID = parsed
		}
		out := make([]map[string]interface{}, 0)
		for _, ownerID := range owners {
			query := `SELECT id,device_id,name,engine,version,port,data_directory,network_access,status,created_at FROM database_servers WHERE user_id=?`
			args := []interface{}{ownerID}
			if projectID > 0 { query += ` AND id IN (SELECT pds.database_server_id FROM project_database_servers pds WHERE pds.project_id=?)`; args = append(args, projectID) }
			if environmentID > 0 { query += ` AND id IN (SELECT eds.database_server_id FROM project_environment_database_servers eds WHERE eds.project_id=? AND eds.environment_id=?)`; args = append(args, projectID, environmentID) }
			query += " ORDER BY created_at DESC"
			rows, err := s.db.conn.QueryContext(r.Context(), query, args...)
			if err != nil { apiError(w, shared.ErrInternal("could not load database servers")); return }
			for rows.Next() {
				var id, deviceID int64
				var name, engine, version, dir, network, status string
				var port int
				var created time.Time
				if err := rows.Scan(&id,&deviceID,&name,&engine,&version,&port,&dir,&network,&status,&created); err != nil { rows.Close(); apiError(w, shared.ErrInternal("could not read database servers")); return }
				out = append(out, map[string]interface{}{"id":id,"device_id":deviceID,"workspace_owner_id":ownerID,"name":name,"engine":engine,"version":version,"port":port,"data_directory":dir,"network_access":network,"status":status,"created_at":created})
			}
			if err := rows.Err(); err != nil { rows.Close(); apiError(w, shared.ErrInternal("could not read database servers")); return }
			rows.Close()
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok":true,"servers":out})
	case http.MethodPost:
		var in databaseServerInput;if err:=decodeJSON(r,&in);err!=nil{apiError(w,shared.ErrBadRequest("invalid JSON"));return}
		in.Name=strings.TrimSpace(in.Name);if in.DeviceID<1||len(in.Name)<2||len(in.Name)>64{apiError(w,shared.ErrBadRequest("device and server name are required"));return}
		ownerID:=u.ID
		if in.ProjectID<0 { apiError(w,shared.ErrBadRequest("invalid project id"));return }
		if in.ProjectID>0 { if err:=s.authorizeProjectAction(r.Context(),in.ProjectID,u.ID,"database.manage");err!=nil{apiError(w,err);return}; if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT user_id FROM projects WHERE id=?`,in.ProjectID).Scan(&ownerID);err!=nil{apiError(w,shared.ErrNotFound("project not found"));return} }
		if in.EnvironmentID<0 || (in.EnvironmentID>0 && in.ProjectID<1) { apiError(w,shared.ErrBadRequest("environment_id requires a project_id"));return }
		if in.EnvironmentID>0 { var envCount int; if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM project_environments WHERE id=? AND project_id=?`,in.EnvironmentID,in.ProjectID).Scan(&envCount);err!=nil||envCount!=1 { apiError(w,shared.ErrNotFound("environment not found"));return } }
		if in.Engine!="postgresql"&&in.Engine!="mariadb"&&in.Engine!="mysql"{apiError(w,shared.ErrBadRequest("unsupported database engine"));return}
		if in.Port<1024||in.Port>65535{apiError(w,shared.ErrBadRequest("invalid port"));return}
		if in.NetworkAccess==""{in.NetworkAccess="private"}
		if in.NetworkAccess!="private"&&in.NetworkAccess!="localhost"{apiError(w,shared.ErrBadRequest("network access must be private or localhost"));return}
		var owned int;if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM devices WHERE id=? AND user_id=?`,in.DeviceID,ownerID).Scan(&owned);err!=nil||owned!=1{apiError(w,shared.ErrNotFound("device not found"));return}
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
		res,err:=s.db.conn.ExecContext(r.Context(),`INSERT INTO database_servers(user_id,device_id,name,engine,version,port,data_directory,network_access,status,storage_limit_gb,ram_budget_mb,max_connections,monitor_storage,auto_start,process_policy,bind_address,ssl_mode,auth_method,remote_access) VALUES(?,?,?,?,?,?,?,?,'planned',?,?,?,?,?,?,?,?,?,?)`,ownerID,in.DeviceID,in.Name,in.Engine,in.Version,in.Port,in.DataDirectory,in.NetworkAccess,in.StorageLimitGB,in.RAMBudgetMB,in.MaxConnections,in.MonitorStorage,in.AutoStart,in.ProcessPolicy,in.BindAddress,in.SSLMode,in.AuthMethod,in.RemoteAccess)
		if err!=nil{apiError(w,shared.ErrBadRequest("server name or port already in use on this device"));return}
		id,_:=res.LastInsertId()
		if in.ProjectID>0 {
			tx,txErr:=s.db.conn.BeginTx(r.Context(),nil)
			if txErr==nil {
				_,txErr=tx.ExecContext(r.Context(),`INSERT IGNORE INTO project_devices(project_id,device_id) VALUES(?,?)`,in.ProjectID,in.DeviceID)
				if txErr==nil { _,txErr=tx.ExecContext(r.Context(),`INSERT IGNORE INTO project_database_servers(project_id,database_server_id) VALUES(?,?)`,in.ProjectID,id) }
				if txErr==nil && in.EnvironmentID>0 { _,txErr=tx.ExecContext(r.Context(),`INSERT IGNORE INTO project_environment_database_servers(project_id,environment_id,database_server_id) VALUES(?,?,?)`,in.ProjectID,in.EnvironmentID,id) }
				if txErr==nil { txErr=tx.Commit() } else { _=tx.Rollback() }
			}
			if txErr!=nil { _,_=s.db.conn.ExecContext(r.Context(),`DELETE FROM database_servers WHERE id=? AND user_id=?`,id,ownerID);apiError(w,shared.ErrInternal("could not attach database server to project"));return }
		}
		writeJSON(w,http.StatusCreated,map[string]interface{}{"ok":true,"server":map[string]interface{}{"id":id,"device_id":in.DeviceID,"name":in.Name,"engine":in.Engine,"version":in.Version,"port":in.Port,"status":"planned","ssl_mode":in.SSLMode,"max_connections":in.MaxConnections},"message":"Configuration saved. Install the PostgreSQL server when the selected device is online."})
	default:http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleInstallDatabaseServer(w http.ResponseWriter,r *http.Request,id int64) {
	u:=UserFromContext(r.Context())
	ownerID,_,accessErr:=s.authorizeDatabaseServer(r.Context(),id,u.ID,"database.manage");if accessErr!=nil{apiError(w,accessErr);return}
	var deviceID int64;var engine,dataDir,status,bind,sslMode string;var port,maxConnections int;var remote bool
	err:=s.db.conn.QueryRowContext(r.Context(),"SELECT device_id,engine,data_directory,status,port,max_connections,bind_address,remote_access,ssl_mode FROM database_servers WHERE id=? AND user_id=?",id,ownerID).Scan(&deviceID,&engine,&dataDir,&status,&port,&maxConnections,&bind,&remote,&sslMode)
	if err==sql.ErrNoRows{apiError(w,shared.ErrNotFound("database server not found"));return};if err!=nil{apiError(w,shared.ErrInternal("could not load database server"));return}
	if engine!="postgresql" && engine!="mariadb" && engine!="mysql"{apiError(w,shared.ErrBadRequest("unsupported database engine"));return}
	if engine=="mariadb" || engine=="mysql" { suffix := "/server-" + fmt.Sprint(id); if !strings.HasSuffix(strings.TrimRight(dataDir, "/"), suffix) { dataDir = strings.TrimRight(dataDir, "/") + suffix }; if _,err:=s.db.conn.ExecContext(r.Context(), "UPDATE database_servers SET data_directory=? WHERE id=? AND user_id=?", dataDir,id,ownerID);err!=nil{apiError(w,shared.ErrInternal("could not reserve isolated MariaDB data directory"));return} }
	if status=="ready"{apiError(w,shared.ErrBadRequest("database server is already ready"));return}
	if status!="planned"&&status!="error"{apiError(w,shared.ErrBadRequest("database server cannot be installed from its current state"));return}
	if bind!="127.0.0.1"||remote{apiError(w,shared.ErrBadRequest("remote database access must remain disabled"));return}
	conn,online:=s.hub.Get(deviceID);if !online{apiError(w,shared.ErrBadRequest("device is offline"));return}
	res,err:=s.db.conn.ExecContext(r.Context(),"UPDATE database_servers SET status='installing' WHERE id=? AND user_id=? AND status IN ('planned','error')",id,ownerID)
	if err!=nil{apiError(w,shared.ErrInternal("could not queue database server installation"));return}
	if n,_:=res.RowsAffected();n!=1{apiError(w,shared.ErrBadRequest("database server state changed; refresh and retry"));return}
	req:=&shared.InstallDatabaseServer{ServerID:id,Engine:engine,Port:uint32(port),DataDirectory:dataDir,MaxConnections:uint32(maxConnections),SSLMode:sslMode}
	go s.runDatabaseServerInstall(id,ownerID,u.ID,deviceID,conn,req)
	writeJSON(w,http.StatusAccepted,map[string]interface{}{"ok":true,"server_id":id,"status":"installing","message":engine+" installation queued on the Android device"})
}

func (s *Server) runDatabaseServerInstall(id,ownerID,actorID,deviceID int64,conn *AgentConn,req *shared.InstallDatabaseServer) {
	ctx,cancel:=context.WithTimeout(context.Background(),10*time.Minute);defer cancel()
	result,err:=conn.SendCommand(ctx,"install_database_server",req)
	if err!=nil || !result.Ok {
		_,_=s.db.conn.ExecContext(context.Background(),"UPDATE database_servers SET status='error' WHERE id=? AND user_id=?",id,ownerID)
		_ = s.db.Log(context.Background(),&shared.LogEntry{UserID:shared.PtrInt64(actorID),DeviceID:shared.PtrInt64(deviceID),Action:"database_server_install_failed",Details:map[string]interface{}{"server_id":id}})
		message:="Database server installation failed";if result!=nil&&result.Message!=""{message=result.Message}else if err!=nil{message=err.Error()}
		s.notifyWorkspaceEvent(context.Background(),ownerID,actorID,"error","Database server installation failed",message)
		return
	}
	var installed shared.DatabaseServerInstallResult
	if err:=shared.Unmarshal(result.Payload,&installed);err!=nil {
		_,_=s.db.conn.ExecContext(context.Background(),"UPDATE database_servers SET status='error' WHERE id=? AND user_id=?",id,ownerID)
		s.notifyWorkspaceEvent(context.Background(),ownerID,actorID,"error","Database server installation failed","The Agent returned an invalid installation result.")
		return
	}
	if _,err:=s.db.conn.ExecContext(context.Background(),"UPDATE database_servers SET status='ready',version=?,data_directory=? WHERE id=? AND user_id=?",installed.Version,installed.DataDirectory,id,ownerID);err!=nil {
		_,_=s.db.conn.ExecContext(context.Background(),"UPDATE database_servers SET status='error' WHERE id=? AND user_id=?",id,ownerID)
		s.notifyWorkspaceEvent(context.Background(),ownerID,actorID,"error","Database server installation failed","The Control Plane could not persist the installation result.")
		return
	}
	_ = s.db.Log(context.Background(),&shared.LogEntry{UserID:shared.PtrInt64(actorID),DeviceID:shared.PtrInt64(deviceID),Action:"database_server_installed",Details:map[string]interface{}{"server_id":id,"engine":req.Engine,"version":installed.Version}})
	s.notifyWorkspaceEvent(context.Background(),ownerID,actorID,"success","Database server ready",fmt.Sprintf("%s %s installed and verified.",req.Engine,installed.Version))
}

func (s *Server) handleDatabaseServerByID(w http.ResponseWriter,r *http.Request) {
	u:=UserFromContext(r.Context());raw:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/databases/servers/"),"/")
	parts:=strings.Split(raw,"/")
	id,err:=strconv.ParseInt(parts[0],10,64);if err!=nil||id<1{apiError(w,shared.ErrBadRequest("invalid server id"));return}
	action:="database.manage";if r.Method==http.MethodGet{action="database.read"};ownerID,_,accessErr:=s.authorizeDatabaseServer(r.Context(),id,u.ID,action);if accessErr!=nil{apiError(w,accessErr);return}
	if len(parts)==2&&parts[1]=="roles"&&r.Method==http.MethodGet {
		var deviceID int64;var engine,status,dataDir string;var port int
		err:=s.db.conn.QueryRowContext(r.Context(),`SELECT device_id,engine,status,port,data_directory FROM database_servers WHERE id=? AND user_id=?`,id,ownerID).Scan(&deviceID,&engine,&status,&port,&dataDir)
		if err==sql.ErrNoRows { apiError(w,shared.ErrNotFound("database server not found"));return }
		if err!=nil { apiError(w,shared.ErrInternal("could not load database server"));return }
		if (engine!="postgresql"&&engine!="mariadb"&&engine!="mysql")||status!="ready" { apiError(w,shared.ErrBadRequest("a supported ready database server is required to inspect users"));return }
		conn,online:=s.hub.Get(deviceID);if !online { apiError(w,shared.ErrBadRequest("device is offline"));return }
		ctx,cancel:=context.WithTimeout(r.Context(),20*time.Second);defer cancel()
		result,err:=conn.SendCommand(ctx,"list_database_roles",&shared.ListDatabaseRoles{Engine:engine,DataDirectory:dataDir,ServerID:id,Port:uint32(port)})
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
	err=s.db.conn.QueryRowContext(r.Context(),`SELECT device_id,name,engine,version,port,data_directory,network_access,status,created_at,storage_limit_gb,ram_budget_mb,max_connections,monitor_storage,auto_start,process_policy,bind_address,ssl_mode,auth_method,remote_access FROM database_servers WHERE id=? AND user_id=?`,id,ownerID).Scan(&did,&name,&engine,&version,&port,&dir,&network,&status,&created,&storage,&ram,&maxConnections,&monitor,&autoStart,&policy,&bind,&ssl,&auth,&remote)
	if err==sql.ErrNoRows{apiError(w,shared.ErrNotFound("database server not found"));return};if err!=nil{apiError(w,shared.ErrInternal("could not load database server"));return}
	switch r.Method{
	case http.MethodGet:writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"server":map[string]interface{}{"id":id,"device_id":did,"name":name,"engine":engine,"version":version,"port":port,"data_directory":dir,"network_access":network,"status":status,"created_at":created,"storage_limit_gb":storage,"ram_budget_mb":ram,"max_connections":maxConnections,"monitor_storage":monitor,"auto_start":autoStart,"process_policy":policy,"bind_address":bind,"ssl_mode":ssl,"auth_method":auth,"remote_access":remote}})
	case http.MethodDelete:
		var count int;if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM db_instances WHERE server_id=?`,id).Scan(&count);err!=nil{apiError(w,shared.ErrInternal("could not inspect databases"));return};if count>0{apiError(w,shared.ErrBadRequest("remove or migrate databases before removing this server"));return}
		_,err:=s.db.conn.ExecContext(r.Context(),`DELETE FROM database_servers WHERE id=? AND user_id=? AND status IN ('planned','error','stopped')`,id,ownerID);if err!=nil{apiError(w,shared.ErrInternal("could not remove server metadata"));return};writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true})
	default:http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleDatabaseServerAction(w http.ResponseWriter,r *http.Request,id int64,action string) {
	u:=UserFromContext(r.Context())
	ownerID,_,accessErr:=s.authorizeDatabaseServer(r.Context(),id,u.ID,"database.manage");if accessErr!=nil{apiError(w,accessErr);return}
	var deviceID int64;var engine,status,dataDir,sslMode string;var port,maxConnections int
	err:=s.db.conn.QueryRowContext(r.Context(),`SELECT device_id,engine,status,data_directory,port,max_connections,ssl_mode FROM database_servers WHERE id=? AND user_id=?`,id,ownerID).Scan(&deviceID,&engine,&status,&dataDir,&port,&maxConnections,&sslMode)
	if err==sql.ErrNoRows { apiError(w,shared.ErrNotFound("database server not found"));return }
	if err!=nil { apiError(w,shared.ErrInternal("could not load database server"));return }
	if engine!="postgresql" && engine!="mariadb" && engine!="mysql" { apiError(w,shared.ErrBadRequest("unsupported database engine"));return }
	if action=="start"&&status!="stopped"&&status!="ready" { apiError(w,shared.ErrBadRequest("only a stopped database server can be started"));return }
	if action=="stop"&&status=="stopped" { writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"status":"stopped","message":engine+" is already stopped"});return }
	if action=="stop"&&status!="ready" { apiError(w,shared.ErrBadRequest("only a ready database server can be stopped"));return }
	conn,online:=s.hub.Get(deviceID);if !online { apiError(w,shared.ErrBadRequest("device is offline"));return }
	previous:=status
	if action=="stop" {
		res,err:=s.db.conn.ExecContext(r.Context(),`UPDATE database_servers SET status='stopping' WHERE id=? AND user_id=? AND status='ready'`,id,ownerID)
		if err!=nil { apiError(w,shared.ErrInternal("could not lock database server state"));return }
		if n,_:=res.RowsAffected();n!=1 { apiError(w,shared.ErrBadRequest("database server state changed; refresh and retry"));return }
	} else {
		res,err:=s.db.conn.ExecContext(r.Context(),`UPDATE database_servers SET status='starting' WHERE id=? AND user_id=? AND status=?`,id,ownerID,status)
		if err!=nil { apiError(w,shared.ErrInternal("could not lock database server state"));return }
		if n,_:=res.RowsAffected();n!=1 { apiError(w,shared.ErrBadRequest("database server state changed; refresh and retry"));return }
	}
	ctx,cancel:=context.WithTimeout(r.Context(),110*time.Second);defer cancel()
	result,err:=conn.SendCommand(ctx,action+"_database_server",&shared.DatabaseServerAction{Engine:engine,MaxConnections:uint32(maxConnections),SSLMode:sslMode,ServerID:id,Port:uint32(port),DataDirectory:dataDir})
	if err!=nil||!result.Ok {
		if previous=="ready"||previous=="stopped" { _,_=s.db.conn.ExecContext(context.Background(),`UPDATE database_servers SET status=? WHERE id=? AND user_id=? AND status IN ('starting','stopping')`,previous,id,ownerID) }
		message:="Agent database lifecycle command failed";if err!=nil{message=err.Error()}else if result!=nil&&result.Message!=""{message=result.Message}
		_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),DeviceID:shared.PtrInt64(deviceID),Action:"database_server_"+action+"_failed",Details:map[string]interface{}{"ok":false,"server_id":id,"error":message},IP:clientIP(r)})
		s.notifyWorkspaceEvent(r.Context(),ownerID,u.ID,"error","Database server "+action+" failed",message)
		if err!=nil { apiError(w,shared.ErrInternal("Agent PostgreSQL lifecycle command failed"));return }
		apiError(w,shared.ErrBadRequest(result.Message));return
	}
	target:="ready";if action=="stop" { target="stopped" }
	if _,err:=s.db.conn.ExecContext(r.Context(),`UPDATE database_servers SET status=? WHERE id=? AND user_id=?`,target,id,ownerID);err!=nil { apiError(w,shared.ErrInternal("Database action succeeded but server status could not be saved"));return }
	_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),DeviceID:shared.PtrInt64(deviceID),Action:"database_server_"+action,Details:map[string]interface{}{"server_id":id}})
	s.notifyWorkspaceEvent(r.Context(),ownerID,u.ID,"success","Database server "+action+" completed",engine+" server "+action+" completed successfully.")
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"status":target,"message":engine+" server "+action+" completed"})
}

func (s *Server) handleDatabases(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	projectID := int64(0)
	environmentID := int64(0)
	var owners []int64
	if raw := r.URL.Query().Get("project_id"); raw != "" {
		parsed, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || parsed < 1 { apiError(w, shared.ErrBadRequest("invalid project id")); return }
		ownerID, accessErr := s.authorizeProjectOwner(r.Context(), parsed, u.ID, "database.read")
		if accessErr != nil { apiError(w, accessErr); return }
		projectID = parsed
		owners = []int64{ownerID}
	} else {
		allOwners, ownerErr := s.db.WorkspaceOwnerIDs(r.Context(), u.ID)
		if ownerErr != nil { apiError(w, ownerErr); return }
		for _, ownerID := range allOwners {
			role, allowed, roleErr := s.db.WorkspaceRole(r.Context(), ownerID, u.ID)
			if roleErr != nil { apiError(w, roleErr); return }
			if allowed && CanWorkspaceAction(role, "database.read") { owners = append(owners, ownerID) }
		}
	}
	if raw := r.URL.Query().Get("environment_id"); raw != "" {
		parsed, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || parsed < 1 || projectID < 1 { apiError(w, shared.ErrBadRequest("environment_id requires a valid project_id")); return }
		var exists int
		if err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM project_environments WHERE id=? AND project_id=?`, parsed, projectID).Scan(&exists); err != nil || exists != 1 { apiError(w, shared.ErrNotFound("environment not found")); return }
		environmentID = parsed
	}
	out := make([]map[string]interface{}, 0)
	for _, ownerID := range owners {
		query := `SELECT d.id,d.server_id,d.name,d.status,d.created_at,ds.name,ds.engine,ds.device_id
			FROM db_instances d JOIN database_servers ds ON ds.id=d.server_id
			WHERE d.user_id=? AND ds.user_id=?`
		args := []interface{}{ownerID, ownerID}
		if projectID > 0 { query += ` AND EXISTS (SELECT 1 FROM project_database_servers pds WHERE pds.project_id=? AND pds.database_server_id=ds.id)`; args = append(args, projectID) }
		if environmentID > 0 { query += ` AND EXISTS (SELECT 1 FROM project_environment_databases edb WHERE edb.project_id=? AND edb.environment_id=? AND edb.database_id=d.id)`; args = append(args, projectID, environmentID) }
		query += " ORDER BY d.created_at DESC"
		rows, err := s.db.conn.QueryContext(r.Context(), query, args...)
		if err != nil { apiError(w, shared.ErrInternal("could not load databases")); return }
		for rows.Next() {
			var id, serverID, deviceID int64
			var name, status, serverName, engine string
			var created time.Time
			if err := rows.Scan(&id,&serverID,&name,&status,&created,&serverName,&engine,&deviceID); err != nil { rows.Close(); apiError(w, shared.ErrInternal("could not read databases")); return }
			out = append(out, map[string]interface{}{"id":id,"server_id":serverID,"name":name,"status":status,"created_at":created,"server_name":serverName,"engine":engine,"device_id":deviceID})
		}
		if err := rows.Err(); err != nil { rows.Close(); apiError(w, shared.ErrInternal("could not read databases")); return }
		rows.Close()
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok":true,"databases":out})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 200 { apiError(w, shared.ErrBadRequest("limit must be between 1 and 200")); return }
		limit = n
	}
	ownerID := u.ID
	projectID := int64(0)
	if raw := r.URL.Query().Get("project_id"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 1 { apiError(w, shared.ErrBadRequest("invalid project id")); return }
		resolved, accessErr := s.authorizeProjectOwner(r.Context(), parsed, u.ID, "project.read")
		if accessErr != nil { apiError(w, accessErr); return }
		projectID, ownerID = parsed, resolved
	}
	query := `SELECT l.id,l.action,l.details,l.ip,l.created_at,COALESCE(d.hostname,'')
		FROM logs l LEFT JOIN devices d ON d.id=l.device_id WHERE l.user_id=?`
	args := []interface{}{ownerID}
	if projectID > 0 {
		query += ` AND (EXISTS (SELECT 1 FROM project_devices pd WHERE pd.project_id=? AND pd.device_id=l.device_id)
			OR EXISTS (SELECT 1 FROM project_sites ps WHERE ps.project_id=? AND ps.device_id=l.device_id))`
		args = append(args, projectID, projectID)
	}
	query += " ORDER BY l.id DESC LIMIT ?"
	args = append(args, limit)
	rows, err := s.db.conn.QueryContext(r.Context(), query, args...)
	if err != nil { apiError(w, shared.ErrInternal("could not load logs")); return }
	defer rows.Close()
	out := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id int64; var action, ip, host string; var details []byte; var created time.Time
		if err := rows.Scan(&id, &action, &details, &ip, &created, &host); err != nil { apiError(w, shared.ErrInternal("could not read logs")); return }
		var parsed interface{}
		if len(details) > 0 { _ = json.Unmarshal(details, &parsed) }
		out = append(out, map[string]interface{}{"id":id,"action":action,"details":parsed,"ip":ip,"created_at":created,"device":host})
	}
	if err := rows.Err(); err != nil { apiError(w, shared.ErrInternal("could not read logs")); return }
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok":true,"logs":out})
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
  if _,ok:=settings["workspace_name"];!ok{settings["workspace_name"]="Personal workspace"}
  if _,ok:=settings["timezone"];!ok{settings["timezone"]="UTC"}
  if _,ok:=settings["date_format"];!ok{settings["date_format"]="locale"}\n\t\tif _,ok:=settings["compact_mode"];!ok{settings["compact_mode"]=false}
  if _,ok:=settings["notifications"];!ok{settings["notifications"]=map[string]bool{"system":true,"deployment_errors":true,"device_events":true,"project_updates":true}}
  writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"settings":settings});return
 }
 if r.Method==http.MethodPut{
  var in map[string]json.RawMessage;if err:=decodeJSON(r,&in);err!=nil{apiError(w,shared.ErrBadRequest("invalid JSON"));return}
  allowed:=map[string]bool{"mode":true,"locale":true,"theme":true,"notifications":true,"workspace_name":true,"timezone":true,"date_format":true,"compact_mode":true}
  for key,raw:=range in{if !allowed[key]{apiError(w,shared.ErrBadRequest("unsupported setting: "+key));return}
   switch key{
   case "mode": var v string;if json.Unmarshal(raw,&v)!=nil||(v!="simple"&&v!="advanced"){apiError(w,shared.ErrBadRequest("mode must be simple or advanced"));return}
   case "locale": var v string;if json.Unmarshal(raw,&v)!=nil||(v!="ru"&&v!="en"){apiError(w,shared.ErrBadRequest("locale must be ru or en"));return}
   case "theme": var v string;if json.Unmarshal(raw,&v)!=nil||(v!="dark"&&v!="light"&&v!="system"){apiError(w,shared.ErrBadRequest("theme must be dark, light or system"));return}
   case "workspace_name": var v string;if json.Unmarshal(raw,&v)!=nil||len(strings.TrimSpace(v))<2||len(v)>80{apiError(w,shared.ErrBadRequest("workspace name must contain 2–80 characters"));return}
   case "timezone": var v string;if json.Unmarshal(raw,&v)!=nil||len(v)>64{apiError(w,shared.ErrBadRequest("invalid timezone"));return};if _,err:=time.LoadLocation(v);err!=nil{apiError(w,shared.ErrBadRequest("timezone must be a valid IANA location"));return}
   case "date_format": var v string;if json.Unmarshal(raw,&v)!=nil||(v!="locale"&&v!="iso"){apiError(w,shared.ErrBadRequest("date format must be locale or iso"));return}\n\t\t\tcase "compact_mode": var v bool;if json.Unmarshal(raw,&v)!=nil{apiError(w,shared.ErrBadRequest("compact mode must be a boolean"));return}
   case "notifications": var v map[string]bool;if json.Unmarshal(raw,&v)!=nil||len(v)>16{apiError(w,shared.ErrBadRequest("notifications must be an object of boolean preferences"));return};for k:=range v{if k!="system"&&k!="deployment_errors"&&k!="device_events"&&k!="project_updates"{apiError(w,shared.ErrBadRequest("unsupported notification preference"));return}}
   }
   _,err:=s.db.conn.ExecContext(r.Context(),`INSERT INTO user_settings(user_id,setting_key,setting_value) VALUES(?,?,?) ON DUPLICATE KEY UPDATE setting_value=VALUES(setting_value)`,u.ID,key,[]byte(raw));if err!=nil{apiError(w,shared.ErrInternal("could not save settings"));return}
  }
  writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true});return
 }
 http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
}

func (s *Server) handleReports(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	ownerID := u.ID
	projectID := int64(0)
	if raw := r.URL.Query().Get("project_id"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 1 { apiError(w, shared.ErrBadRequest("invalid project id")); return }
		resolved, accessErr := s.authorizeProjectOwner(r.Context(), parsed, u.ID, "project.read")
		if accessErr != nil { apiError(w, accessErr); return }
		projectID, ownerID = parsed, resolved
	}
	days, periodErr := parseReportPeriodDays(r.URL.Query().Get("days"))
	if periodErr != nil { apiError(w, periodErr); return }
	var devices, sites, running, dbServers int
	if projectID > 0 {
		if err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM project_devices pd JOIN devices d ON d.id=pd.device_id WHERE pd.project_id=? AND d.user_id=?`, projectID, ownerID).Scan(&devices); err != nil { apiError(w, shared.ErrInternal("could not count project devices")); return }
		if err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM project_sites ps JOIN sites s ON s.device_id=ps.device_id AND s.site_id=ps.site_id WHERE ps.project_id=?`, projectID).Scan(&sites); err != nil { apiError(w, shared.ErrInternal("could not count project websites")); return }
		if err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM project_sites ps JOIN sites s ON s.device_id=ps.device_id AND s.site_id=ps.site_id WHERE ps.project_id=? AND s.status='running'`, projectID).Scan(&running); err != nil { apiError(w, shared.ErrInternal("could not count running websites")); return }
		if err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM project_database_servers pds JOIN database_servers ds ON ds.id=pds.database_server_id WHERE pds.project_id=? AND ds.user_id=?`, projectID, ownerID).Scan(&dbServers); err != nil { apiError(w, shared.ErrInternal("could not count database servers")); return }
	} else {
		if err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM devices WHERE user_id=?`, ownerID).Scan(&devices); err != nil { apiError(w, shared.ErrInternal("could not count devices")); return }
		if err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM sites s JOIN devices d ON d.id=s.device_id WHERE d.user_id=?`, ownerID).Scan(&sites); err != nil { apiError(w, shared.ErrInternal("could not count websites")); return }
		if err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM sites s JOIN devices d ON d.id=s.device_id WHERE d.user_id=? AND s.status='running'`, ownerID).Scan(&running); err != nil { apiError(w, shared.ErrInternal("could not count running websites")); return }
		if err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM database_servers WHERE user_id=?`, ownerID).Scan(&dbServers); err != nil { apiError(w, shared.ErrInternal("could not count database servers")); return }
	}
	var buildJobs, buildSucceeded, buildFailed int
	if projectID > 0 {
		err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*),COALESCE(SUM(b.status='succeeded'),0),COALESCE(SUM(b.status='failed'),0)
			FROM build_jobs b WHERE b.user_id=? AND b.created_at>=DATE_SUB(UTC_TIMESTAMP(),INTERVAL ? DAY)
			AND EXISTS(SELECT 1 FROM project_sites ps WHERE ps.project_id=? AND ps.device_id=b.device_id AND ps.site_id=b.site_id)`, ownerID, days, projectID).Scan(&buildJobs,&buildSucceeded,&buildFailed)
		if err != nil { apiError(w, shared.ErrInternal("could not load project build analytics")); return }
	} else {
		err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*),COALESCE(SUM(status='succeeded'),0),COALESCE(SUM(status='failed'),0)
			FROM build_jobs WHERE user_id=? AND created_at>=DATE_SUB(UTC_TIMESTAMP(),INTERVAL ? DAY)`, ownerID, days).Scan(&buildJobs,&buildSucceeded,&buildFailed)
		if err != nil { apiError(w, shared.ErrInternal("could not load build analytics")); return }
	}
	var backups, backupsCompleted, backupsFailed int
	if projectID > 0 {
		err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*),COALESCE(SUM(b.status IN ('completed','verified')),0),COALESCE(SUM(b.status='failed'),0)
			FROM backups b WHERE b.user_id=? AND b.created_at>=DATE_SUB(UTC_TIMESTAMP(),INTERVAL ? DAY)
			AND ((b.resource_type='site' AND EXISTS(SELECT 1 FROM project_sites ps WHERE ps.project_id=? AND ps.device_id=b.device_id AND b.resource_id=CONCAT(ps.device_id,':',ps.site_id)))
			OR (b.resource_type='database' AND EXISTS(SELECT 1 FROM project_database_servers pds JOIN db_instances d ON d.server_id=pds.database_server_id WHERE pds.project_id=? AND d.user_id=b.user_id AND CAST(d.id AS CHAR)=b.resource_id)))`, ownerID, days, projectID, projectID).Scan(&backups,&backupsCompleted,&backupsFailed)
		if err != nil { apiError(w, shared.ErrInternal("could not load project backup analytics")); return }
	} else {
		err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*),COALESCE(SUM(status IN ('completed','verified')),0),COALESCE(SUM(status='failed'),0)
			FROM backups WHERE user_id=? AND created_at>=DATE_SUB(UTC_TIMESTAMP(),INTERVAL ? DAY)`, ownerID, days).Scan(&backups,&backupsCompleted,&backupsFailed)
		if err != nil { apiError(w, shared.ErrInternal("could not load backup analytics")); return }
	}
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"report":map[string]interface{}{
		"generated_at":time.Now().UTC(),"period_days":days,"devices":devices,"sites":sites,"running_sites":running,"database_servers":dbServers,
		"build_jobs":buildJobs,"build_succeeded":buildSucceeded,"build_failed":buildFailed,
		"backups":backups,"backups_completed":backupsCompleted,"backups_failed":backupsFailed,
	}})
}

func (s *Server) handleBackups(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if r.Method == http.MethodPost {
		var in struct { DeviceID int64 `json:"device_id"`; SiteID string `json:"site_id"`; DatabaseID int64 `json:"database_id"` }
		if err := decodeJSON(r, &in); err != nil { apiError(w, shared.ErrBadRequest("invalid backup request")); return }
		if in.DatabaseID > 0 {
			ownerID, serverID, deviceID, accessErr := s.authorizeDatabaseInstance(r.Context(), in.DatabaseID, u.ID, "database.manage")
			if accessErr != nil { apiError(w, accessErr); return }
			var database, engine, databaseStatus, serverStatus, dataDir string
			var port int
			err := s.db.conn.QueryRowContext(r.Context(), `SELECT d.name,ds.engine,d.status,ds.status,ds.port,ds.data_directory
				FROM db_instances d JOIN database_servers ds ON ds.id=d.server_id
				WHERE d.id=? AND d.user_id=?`, in.DatabaseID, ownerID).Scan(&database, &engine, &databaseStatus, &serverStatus, &port, &dataDir)
			if err != nil { apiError(w, shared.ErrInternal("could not load database backup target")); return }
			if (engine != "postgresql" && engine != "mariadb" && engine != "mysql") || databaseStatus != "ready" || serverStatus != "ready" { apiError(w, shared.ErrBadRequest("database and server must be ready")); return }
			conn, online := s.hub.Get(deviceID)
			if !online { apiError(w, shared.ErrBadRequest("device is offline")); return }
			res, err := s.db.conn.ExecContext(r.Context(), `INSERT INTO backups(user_id,device_id,database_server_id,resource_type,resource_id,file_name,size_bytes,status) VALUES(?,?,?,?,?,?,0,'queued')`, ownerID, deviceID, serverID, "database", fmt.Sprint(in.DatabaseID), fmt.Sprintf("pending-%d", in.DatabaseID))
			if err != nil { apiError(w, shared.ErrInternal("could not queue database backup")); return }
			backupID, _ := res.LastInsertId()
			go s.runDatabaseBackup(backupID, ownerID, u.ID, deviceID, conn, &shared.CreateDatabaseBackup{DatabaseID: in.DatabaseID, ServerID: serverID, Engine:engine,DataDirectory:dataDir,Database: database, Port: uint32(port)})
			writeJSON(w, http.StatusAccepted, map[string]interface{}{"ok": true, "backup": map[string]interface{}{"id": backupID, "device_id": deviceID, "resource_type": "database", "resource_id": fmt.Sprint(in.DatabaseID), "file_name": fmt.Sprintf("pending-%d", in.DatabaseID), "size_bytes": 0, "status": "queued"}})
			return
		}
		if in.DeviceID < 1 || !validBackupSiteID(in.SiteID) { apiError(w, shared.ErrBadRequest("invalid device or site")); return }
		ownerID, accessErr := s.authorizeDeviceOwner(r.Context(), in.DeviceID, u.ID, "backup.manage")
		if accessErr != nil { apiError(w, accessErr); return }
		var count int
		if err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM sites st JOIN devices d ON d.id=st.device_id WHERE st.device_id=? AND st.site_id=? AND d.user_id=?`, in.DeviceID, in.SiteID, ownerID).Scan(&count); err != nil || count != 1 {
			apiError(w, shared.ErrNotFound("site not found")); return
		}
		conn, online := s.hub.Get(in.DeviceID)
		if !online { apiError(w, shared.ErrBadRequest("device is offline")); return }
		resourceID := fmt.Sprintf("%d:%s", in.DeviceID, in.SiteID)
		pendingName := fmt.Sprintf("pending-%s.tar.gz", in.SiteID)
		res, err := s.db.conn.ExecContext(r.Context(), "INSERT INTO backups(user_id,device_id,resource_type,resource_id,file_name,size_bytes,status) VALUES(?,?,?,?,?,0,'queued')", ownerID, in.DeviceID, "site", resourceID, pendingName)
		if err != nil { apiError(w, shared.ErrInternal("could not queue website backup")); return }
		id, _ := res.LastInsertId()
		go s.runSiteBackup(id, ownerID, u.ID, in.DeviceID, conn, &shared.CreateSiteBackup{SiteId: in.SiteID})
		writeJSON(w, http.StatusAccepted, map[string]interface{}{"ok": true, "backup": map[string]interface{}{"id": id, "device_id": in.DeviceID, "resource_type": "site", "resource_id": resourceID, "file_name": pendingName, "size_bytes": 0, "status": "queued"}})
		return
	}
	if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	owners, err := s.db.WorkspaceOwnerIDs(r.Context(), u.ID)
	if err != nil { apiError(w, err); return }
	out := make([]map[string]interface{}, 0)
	for _, ownerID := range owners {
		role, allowed, roleErr := s.db.WorkspaceRole(r.Context(), ownerID, u.ID)
		if roleErr != nil { apiError(w, roleErr); return }
		if !allowed || !CanWorkspaceAction(role, "backup.read") { continue }
		rows, queryErr := s.db.conn.QueryContext(r.Context(), `SELECT id,device_id,resource_type,resource_id,file_name,size_bytes,status,created_at FROM backups WHERE user_id=? ORDER BY id DESC LIMIT 100`, ownerID)
		if queryErr != nil { apiError(w, shared.ErrInternal("could not load backups")); return }
		for rows.Next() {
			var id, deviceID, size int64
			var typ, resource, file, status string
			var created time.Time
			if err := rows.Scan(&id, &deviceID, &typ, &resource, &file, &size, &status, &created); err != nil { rows.Close(); apiError(w, shared.ErrInternal("could not read backups")); return }
			out = append(out, map[string]interface{}{"id": id, "device_id": deviceID, "resource_type": typ, "resource_id": resource, "file_name": file, "size_bytes": size, "status": status, "created_at": created})
		}
		if err := rows.Err(); err != nil { rows.Close(); apiError(w, shared.ErrInternal("could not read backups")); return }
		rows.Close()
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "backups": out})
}

func (s *Server) runSiteBackup(backupID,ownerID,actorID,deviceID int64,conn *AgentConn,req *shared.CreateSiteBackup) {
	_,_=s.db.conn.ExecContext(context.Background(),"UPDATE backups SET status='running' WHERE id=? AND user_id=?",backupID,ownerID)
	ctx,cancel:=context.WithTimeout(context.Background(),2*time.Minute);defer cancel()
	result,err:=conn.SendCommand(ctx,"create_site_backup",req)
	if err!=nil||!result.Ok {
		_,_=s.db.conn.ExecContext(context.Background(),"UPDATE backups SET status='failed' WHERE id=? AND user_id=?",backupID,ownerID)
		message:="Agent could not create the website backup";if err!=nil{message=err.Error()}else if result!=nil&&result.Message!=""{message=result.Message}
		_ = s.db.Log(context.Background(),&shared.LogEntry{UserID:shared.PtrInt64(actorID),DeviceID:shared.PtrInt64(deviceID),Action:"site_backup_failed",Details:map[string]interface{}{"ok":false,"backup_id":backupID,"error":message}})
		s.notifyWorkspaceEvent(context.Background(),ownerID,actorID,"error","Website backup failed",message);return
	}
	var backup shared.SiteBackupResult
	if err:=shared.Unmarshal(result.Payload,&backup);err!=nil||backup.FileName==""||backup.SizeBytes<1 {
		_,_=s.db.conn.ExecContext(context.Background(),"UPDATE backups SET status='failed' WHERE id=? AND user_id=?",backupID,ownerID)
		_ = s.db.Log(context.Background(),&shared.LogEntry{UserID:shared.PtrInt64(actorID),DeviceID:shared.PtrInt64(deviceID),Action:"site_backup_failed",Details:map[string]interface{}{"ok":false,"backup_id":backupID,"error":"invalid backup metadata"}})
		s.notifyWorkspaceEvent(context.Background(),ownerID,actorID,"error","Website backup failed","Agent returned invalid backup metadata.");return
	}
	if _,err:=s.db.conn.ExecContext(context.Background(),"UPDATE backups SET file_name=?,size_bytes=?,status='completed' WHERE id=? AND user_id=?",backup.FileName,backup.SizeBytes,backupID,ownerID);err!=nil {
		_ = s.db.Log(context.Background(),&shared.LogEntry{UserID:shared.PtrInt64(actorID),DeviceID:shared.PtrInt64(deviceID),Action:"site_backup_failed",Details:map[string]interface{}{"ok":false,"backup_id":backupID,"error":"metadata save failed"}})
		s.notifyWorkspaceEvent(context.Background(),ownerID,actorID,"error","Website backup failed","The Control Plane could not save backup metadata.");return
	}
	_ = s.db.Log(context.Background(),&shared.LogEntry{UserID:shared.PtrInt64(actorID),DeviceID:shared.PtrInt64(deviceID),Action:"site_backup_created",Details:map[string]interface{}{"backup_id":backupID,"site_id":req.SiteId,"size_bytes":backup.SizeBytes}})
	s.notifyWorkspaceEvent(context.Background(),ownerID,actorID,"success","Website backup completed",fmt.Sprintf("Backup %s is ready (%d bytes).",backup.FileName,backup.SizeBytes))
}

func (s *Server) runDatabaseBackup(backupID,ownerID,actorID,deviceID int64,conn *AgentConn,req *shared.CreateDatabaseBackup) {
	_,_=s.db.conn.ExecContext(context.Background(),`UPDATE backups SET status='running' WHERE id=? AND user_id=?`,backupID,ownerID)
	ctx,cancel:=context.WithTimeout(context.Background(),13*time.Minute);defer cancel()
	result,err:=conn.SendCommand(ctx,"create_database_backup",req)
	if err!=nil||!result.Ok {
		_,_=s.db.conn.ExecContext(context.Background(),`UPDATE backups SET status='failed' WHERE id=? AND user_id=?`,backupID,ownerID)
		message:="Agent could not create the database backup";if err!=nil{message=err.Error()}else if result!=nil&&result.Message!=""{message=result.Message}
		_ = s.db.Log(context.Background(),&shared.LogEntry{UserID:shared.PtrInt64(actorID),DeviceID:shared.PtrInt64(deviceID),Action:"database_backup_failed",Details:map[string]interface{}{"ok":false,"backup_id":backupID,"error":message}})
		s.notifyWorkspaceEvent(context.Background(),ownerID,actorID,"error","Database backup failed",message);return
	}
	var backup shared.DatabaseBackupResult
	if err:=shared.Unmarshal(result.Payload,&backup);err!=nil||backup.FileName==""||backup.SizeBytes<1||!backup.Verified {
		_,_=s.db.conn.ExecContext(context.Background(),`UPDATE backups SET status='failed' WHERE id=? AND user_id=?`,backupID,ownerID)
		_ = s.db.Log(context.Background(),&shared.LogEntry{UserID:shared.PtrInt64(actorID),DeviceID:shared.PtrInt64(deviceID),Action:"database_backup_failed",Details:map[string]interface{}{"ok":false,"backup_id":backupID,"error":"backup not verified"}})
		s.notifyWorkspaceEvent(context.Background(),ownerID,actorID,"error","Database backup failed","Agent did not return a verified backup.");return
	}
	if _,err:=s.db.conn.ExecContext(context.Background(),`UPDATE backups SET file_name=?,size_bytes=?,status='verified' WHERE id=? AND user_id=?`,backup.FileName,backup.SizeBytes,backupID,ownerID);err!=nil {
		_ = s.db.Log(context.Background(),&shared.LogEntry{UserID:shared.PtrInt64(actorID),DeviceID:shared.PtrInt64(deviceID),Action:"database_backup_failed",Details:map[string]interface{}{"ok":false,"backup_id":backupID,"error":"metadata save failed"}})
		s.notifyWorkspaceEvent(context.Background(),ownerID,actorID,"error","Database backup failed","The Control Plane could not save backup metadata.");return
	}
	_ = s.db.Log(context.Background(),&shared.LogEntry{UserID:shared.PtrInt64(actorID),DeviceID:shared.PtrInt64(deviceID),Action:"database_backup_created",Details:map[string]interface{}{"backup_id":backupID,"database_id":req.DatabaseID,"size_bytes":backup.SizeBytes}})
	s.notifyWorkspaceEvent(context.Background(),ownerID,actorID,"success","Database backup verified",fmt.Sprintf("Backup %s is verified (%d bytes).",backup.FileName,backup.SizeBytes))
}

func validBackupSiteID(siteID string) bool {
	if len(siteID)<1||len(siteID)>64{return false}
	for _,r:=range siteID{if !((r>='a'&&r<='z')||(r>='A'&&r<='Z')||(r>='0'&&r<='9')||r=='-'||r=='_'){return false}}
	return true
}

var _ = fmt.Sprintf

func (s *Server) handleDomains(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	ownerID := u.ID
	projectID := int64(0)
	if raw := r.URL.Query().Get("project_id"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 1 { apiError(w, shared.ErrBadRequest("invalid project id")); return }
		resolved, accessErr := s.authorizeProjectOwner(r.Context(), parsed, u.ID, "project.read")
		if accessErr != nil { apiError(w, accessErr); return }
		projectID, ownerID = parsed, resolved
	}
	query := `SELECT st.device_id,st.site_id,st.domain,st.port,st.status,COALESCE(d.hostname,'Android host')
		FROM sites st JOIN devices d ON d.id=st.device_id
		WHERE d.user_id=? AND st.domain IS NOT NULL AND st.domain<>''`
	args := []interface{}{ownerID}
	if projectID > 0 { query += ` AND EXISTS (SELECT 1 FROM project_sites ps WHERE ps.project_id=? AND ps.device_id=st.device_id AND ps.site_id=st.site_id)`; args = append(args, projectID) }
	query += " ORDER BY st.updated_at DESC"
	rows, err := s.db.conn.QueryContext(r.Context(), query, args...)
	if err != nil { apiError(w, shared.ErrInternal("could not load domains")); return }
	defer rows.Close()
	out := make([]map[string]interface{}, 0)
	for rows.Next() {
		var deviceID int64; var siteID, domain, status, hostname string; var port int
		if err := rows.Scan(&deviceID, &siteID, &domain, &port, &status, &hostname); err != nil { apiError(w, shared.ErrInternal("could not read domains")); return }
		out = append(out, map[string]interface{}{"device_id":deviceID,"site_id":siteID,"domain":domain,"port":port,"site_status":status,"device_name":hostname,"routing_status":"not_verified"})
	}
	if err := rows.Err(); err != nil { apiError(w, shared.ErrInternal("could not read domains")); return }
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok":true,"domains":out})
}

func (s *Server) handleDeployments(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	ownerID := u.ID
	projectID := int64(0)
	if raw := r.URL.Query().Get("project_id"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 1 { apiError(w, shared.ErrBadRequest("invalid project id")); return }
		resolved, accessErr := s.authorizeProjectOwner(r.Context(), parsed, u.ID, "project.read")
		if accessErr != nil { apiError(w, accessErr); return }
		projectID, ownerID = parsed, resolved
	}
	query := `SELECT l.id,l.action,l.details,l.created_at,COALESCE(d.hostname,'')
		FROM logs l LEFT JOIN devices d ON d.id=l.device_id
		WHERE l.user_id=? AND l.action IN ('site_deploy','site_rebuild','site_rollback','site_restart')`
	args := []interface{}{ownerID}
	if projectID > 0 {
		query += ` AND EXISTS (SELECT 1 FROM project_sites ps WHERE ps.project_id=? AND ps.device_id=l.device_id AND ps.site_id=JSON_UNQUOTE(JSON_EXTRACT(l.details,'$.site_id')))`
		args = append(args, projectID)
	}
	query += " ORDER BY l.id DESC LIMIT 100"
	rows, err := s.db.conn.QueryContext(r.Context(), query, args...)
	if err != nil { apiError(w, shared.ErrInternal("could not load deployment history")); return }
	defer rows.Close()
	out := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id int64; var action, hostname string; var details []byte; var created time.Time
		if err := rows.Scan(&id, &action, &details, &created, &hostname); err != nil { apiError(w, shared.ErrInternal("could not read deployment history")); return }
		var parsed interface{}
		if len(details) > 0 { _ = json.Unmarshal(details, &parsed) }
		out = append(out, map[string]interface{}{"id":id,"action":action,"details":parsed,"created_at":created,"device":hostname})
	}
	if err := rows.Err(); err != nil { apiError(w, shared.ErrInternal("could not read deployment history")); return }
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok":true,"deployments":out})
}

func (s *Server) handleMonitoring(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	ownerID := u.ID
	projectID := int64(0)
	if raw := r.URL.Query().Get("project_id"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 1 { apiError(w, shared.ErrBadRequest("invalid project id")); return }
		resolved, accessErr := s.authorizeProjectOwner(r.Context(), parsed, u.ID, "project.read")
		if accessErr != nil { apiError(w, accessErr); return }
		projectID, ownerID = parsed, resolved
	}
	var totalDevices, onlineDevices, totalSites, runningSites, totalDB int
	var rows *sql.Rows
	var err error
	if projectID > 0 {
		_ = s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM project_devices pd JOIN devices d ON d.id=pd.device_id WHERE pd.project_id=? AND d.user_id=?`, projectID, ownerID).Scan(&totalDevices)
		_ = s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM project_devices pd JOIN devices d ON d.id=pd.device_id WHERE pd.project_id=? AND d.user_id=? AND d.status='online'`, projectID, ownerID).Scan(&onlineDevices)
		_ = s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM project_sites ps JOIN sites st ON st.device_id=ps.device_id AND st.site_id=ps.site_id WHERE ps.project_id=?`, projectID).Scan(&totalSites)
		_ = s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM project_sites ps JOIN sites st ON st.device_id=ps.device_id AND st.site_id=ps.site_id WHERE ps.project_id=? AND st.status='running'`, projectID).Scan(&runningSites)
		_ = s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM project_database_servers pds JOIN database_servers ds ON ds.id=pds.database_server_id WHERE pds.project_id=? AND ds.user_id=?`, projectID, ownerID).Scan(&totalDB)
		rows, err = s.db.conn.QueryContext(r.Context(), `SELECT d.id,COALESCE(d.hostname,'Android host'),d.status,d.agent_version,d.last_seen,
			(SELECT COUNT(*) FROM project_sites ps WHERE ps.project_id=? AND ps.device_id=d.id),
			(SELECT COUNT(*) FROM project_sites ps JOIN sites st ON st.device_id=ps.device_id AND st.site_id=ps.site_id WHERE ps.project_id=? AND ps.device_id=d.id AND st.status='running'),
			m.cpu_percent,m.load_1m,m.memory_total_bytes,m.memory_available_bytes,m.storage_total_bytes,m.storage_available_bytes,m.temperature_celsius,m.captured_at
			FROM project_devices pd JOIN devices d ON d.id=pd.device_id
			LEFT JOIN device_metrics m ON m.id=(SELECT MAX(mx.id) FROM device_metrics mx WHERE mx.device_id=d.id)
			WHERE pd.project_id=? AND d.user_id=? ORDER BY d.last_seen DESC`, projectID, projectID, projectID, ownerID)
	} else {
		_ = s.db.conn.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM devices WHERE user_id=?", ownerID).Scan(&totalDevices)
		_ = s.db.conn.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM devices WHERE user_id=? AND status='online'", ownerID).Scan(&onlineDevices)
		_ = s.db.conn.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM sites st JOIN devices d ON d.id=st.device_id WHERE d.user_id=?", ownerID).Scan(&totalSites)
		_ = s.db.conn.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM sites st JOIN devices d ON d.id=st.device_id WHERE d.user_id=? AND st.status='running'", ownerID).Scan(&runningSites)
		_ = s.db.conn.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM database_servers WHERE user_id=?", ownerID).Scan(&totalDB)
		rows, err = s.db.conn.QueryContext(r.Context(), `SELECT d.id,COALESCE(d.hostname,'Android host'),d.status,d.agent_version,d.last_seen,
			(SELECT COUNT(*) FROM sites st WHERE st.device_id=d.id),
			(SELECT COUNT(*) FROM sites st WHERE st.device_id=d.id AND st.status='running'),
			m.cpu_percent,m.load_1m,m.memory_total_bytes,m.memory_available_bytes,m.storage_total_bytes,m.storage_available_bytes,m.temperature_celsius,m.captured_at
			FROM devices d LEFT JOIN device_metrics m ON m.id=(SELECT MAX(mx.id) FROM device_metrics mx WHERE mx.device_id=d.id)
			WHERE d.user_id=? ORDER BY d.last_seen DESC`, ownerID)
	}
	if err != nil { apiError(w, shared.ErrInternal("could not load monitoring data")); return }
	defer rows.Close()
	devices := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id int64; var hostname, status, version string; var lastSeen, metricAt sql.NullTime; var sites, running int
		var cpu, load, temp sql.NullFloat64; var memTotal, memAvailable, storageTotal, storageAvailable sql.NullInt64
		if err := rows.Scan(&id,&hostname,&status,&version,&lastSeen,&sites,&running,&cpu,&load,&memTotal,&memAvailable,&storageTotal,&storageAvailable,&temp,&metricAt); err != nil { apiError(w, shared.ErrInternal("could not read monitoring data")); return }
		var seen, metricsAt interface{}
		if lastSeen.Valid { seen = lastSeen.Time }
		if metricAt.Valid { metricsAt = metricAt.Time }
		metrics := map[string]interface{}{}
		if cpu.Valid { metrics["cpu_percent"] = cpu.Float64 }
		if load.Valid { metrics["load_1m"] = load.Float64 }
		if memTotal.Valid { metrics["memory_total_bytes"] = memTotal.Int64 }
		if memAvailable.Valid { metrics["memory_available_bytes"] = memAvailable.Int64 }
		if storageTotal.Valid { metrics["storage_total_bytes"] = storageTotal.Int64 }
		if storageAvailable.Valid { metrics["storage_available_bytes"] = storageAvailable.Int64 }
		if temp.Valid { metrics["temperature_celsius"] = temp.Float64 }
		if metricsAt != nil { metrics["captured_at"] = metricsAt }
		history, historyErr := s.db.ListDeviceMetricHistory(r.Context(), id, 20)
		if historyErr != nil { history = []DeviceMetricPoint{} }
		devices = append(devices, map[string]interface{}{"id":id,"hostname":hostname,"status":status,"agent_version":version,"last_seen":seen,"site_count":sites,"running_sites":running,"metrics":metrics,"history":history})
	}
	if err := rows.Err(); err != nil { apiError(w, shared.ErrInternal("could not read monitoring data")); return }
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok":true,"summary":map[string]int{"devices":totalDevices,"online_devices":onlineDevices,"sites":totalSites,"running_sites":runningSites,"database_servers":totalDB},"devices":devices})
}

type restoreDatabaseBackupInput struct {
	BackupID int64 `json:"backup_id"`
	Name string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password"`
	SiteID string `json:"site_id"`
	ProjectID int64 `json:"project_id"`
	EnvironmentID int64 `json:"environment_id"`
}

func (s *Server) handleRestoreDatabaseBackup(w http.ResponseWriter,r *http.Request) {
	if r.Method!=http.MethodPost { http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return }
	u:=UserFromContext(r.Context())
	var in restoreDatabaseBackupInput
	if err:=decodeJSON(r,&in);err!=nil { apiError(w,shared.ErrBadRequest("invalid restore request"));return }
	in.Name=strings.TrimSpace(in.Name);in.Username=strings.TrimSpace(in.Username)
	if in.BackupID<1||!postgresIdentifier.MatchString(in.Name)||!postgresIdentifier.MatchString(in.Username)||len(in.Password)<12||len(in.Password)>256||strings.IndexByte(in.Password,0)>=0||!validProjectEnvironmentScope(in.ProjectID,in.EnvironmentID) {
		apiError(w,shared.ErrBadRequest("provide a valid backup, database name, username and password (12–256 characters)"));return
	}
	ownerID,_,accessErr:=s.authorizeBackup(r.Context(),in.BackupID,u.ID,"backup.manage");if accessErr!=nil{apiError(w,accessErr);return}
	if in.ProjectID > 0 {
		projectOwnerID, projectErr := s.authorizeProjectOwner(r.Context(), in.ProjectID, u.ID, "database.manage")
		if projectErr != nil { apiError(w, projectErr); return }
		if projectOwnerID != ownerID { apiError(w, shared.ErrNotFound("project not found")); return }
	}
	var deviceID int64;var backupServerID sql.NullInt64;var fileName,backupStatus,resourceType string
	err:=s.db.conn.QueryRowContext(r.Context(),`SELECT device_id,database_server_id,file_name,status,resource_type FROM backups WHERE id=? AND user_id=?`,in.BackupID,ownerID).Scan(&deviceID,&backupServerID,&fileName,&backupStatus,&resourceType)
	if err==sql.ErrNoRows { apiError(w,shared.ErrNotFound("database backup not found"));return }
	if err!=nil { apiError(w,shared.ErrInternal("could not load database backup"));return }
	if resourceType!="database"||backupStatus!="verified"||!backupServerID.Valid { apiError(w,shared.ErrBadRequest("only verified database backups can be restored"));return }
	var engine,serverStatus,dataDir,sslMode string;var port int
	err=s.db.conn.QueryRowContext(r.Context(),`SELECT engine,status,port,data_directory,ssl_mode FROM database_servers WHERE id=? AND user_id=? AND device_id=?`,backupServerID.Int64,ownerID,deviceID).Scan(&engine,&serverStatus,&port,&dataDir,&sslMode)
	if err==sql.ErrNoRows { apiError(w,shared.ErrNotFound("source Database Server is no longer available"));return }
	if err!=nil { apiError(w,shared.ErrInternal("could not load restore target server"));return }
	if (engine!="postgresql"&&engine!="mariadb"&&engine!="mysql")||serverStatus!="ready" { apiError(w,shared.ErrBadRequest("restore requires the original database server to be ready"));return }
	if in.ProjectID > 0 {
		var assigned int
		err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM project_database_servers pds
			JOIN project_environment_database_servers eds ON eds.project_id=pds.project_id AND eds.database_server_id=pds.database_server_id
			WHERE pds.project_id=? AND eds.environment_id=? AND pds.database_server_id=?`, in.ProjectID, in.EnvironmentID, backupServerID.Int64).Scan(&assigned)
		if err != nil || assigned != 1 { apiError(w, shared.ErrNotFound("source Database Server is not assigned to this project environment")); return }
	}
	if in.SiteID!="" {
		if !validBackupSiteID(in.SiteID) { apiError(w,shared.ErrBadRequest("invalid website id"));return }
		var count int;if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM sites st JOIN devices d ON d.id=st.device_id WHERE st.device_id=? AND st.site_id=? AND d.user_id=?`,deviceID,in.SiteID,ownerID).Scan(&count);err!=nil||count!=1 { apiError(w,shared.ErrNotFound("website not found on source device"));return }
		if in.ProjectID > 0 {
			var scoped int
			err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM project_environment_sites WHERE project_id=? AND environment_id=? AND device_id=? AND site_id=?`, in.ProjectID, in.EnvironmentID, deviceID, in.SiteID).Scan(&scoped)
			if err != nil || scoped != 1 { apiError(w, shared.ErrNotFound("website is not assigned to the selected project environment")); return }
		}
	}
	var duplicate int
	if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM db_instances WHERE server_id=? AND name=?`,backupServerID.Int64,in.Name).Scan(&duplicate);err!=nil { apiError(w,shared.ErrInternal("could not validate target database name"));return }
	if duplicate>0 { apiError(w,shared.ErrBadRequest("target database name already exists"));return }
	conn,online:=s.hub.Get(deviceID);if !online { apiError(w,shared.ErrBadRequest("device is offline"));return }
	nonce,ciphertext,err:=encryptCredential(s.credentialKey,[]byte(in.Password));if err!=nil { apiError(w,shared.ErrInternal("could not protect target database credentials"));return }
	res,err:=s.db.conn.ExecContext(r.Context(),`INSERT INTO db_instances(user_id,server_id,name,status) VALUES(?,?,?,'planned')`,ownerID,backupServerID.Int64,in.Name)
	if err!=nil { apiError(w,shared.ErrBadRequest("target database name already exists or could not be reserved"));return }
	targetID,_:=res.LastInsertId()
	jobRes,err:=s.db.conn.ExecContext(r.Context(),`INSERT INTO database_restore_jobs(user_id,backup_id,target_database_id,status) VALUES(?,?,?,'queued')`,ownerID,in.BackupID,targetID)
	if err!=nil { _,_=s.db.conn.ExecContext(context.Background(),`DELETE FROM db_instances WHERE id=? AND user_id=?`,targetID,ownerID);apiError(w,shared.ErrInternal("could not queue restore job"));return }
	jobID,_:=jobRes.LastInsertId()
	req:=&shared.RestoreDatabaseBackup{BackupServerID:backupServerID.Int64,BackupFileName:fileName,Database:shared.CreateDatabase{Engine:engine,SSLMode:sslMode,DataDirectory:dataDir,DatabaseID:targetID,ServerID:backupServerID.Int64,Database:in.Name,Username:in.Username,Password:in.Password,Port:uint32(port)}}
	go s.runDatabaseRestore(jobID,in.BackupID,targetID,ownerID,u.ID,deviceID,in.ProjectID,in.EnvironmentID,conn,req,in.SiteID,nonce,ciphertext)
	writeJSON(w,http.StatusAccepted,map[string]interface{}{"ok":true,"job_id":jobID,"database":map[string]interface{}{"id":targetID,"name":in.Name,"status":"planned"},"status":"queued","message":"Backup will be restored into a new database; the source database and backup remain untouched."})
}

func (s *Server) runDatabaseRestore(jobID,backupID,targetID,ownerID,actorID,deviceID,projectID,environmentID int64,conn *AgentConn,req *shared.RestoreDatabaseBackup,siteID string,nonce,ciphertext []byte) {
	_,_=s.db.conn.ExecContext(context.Background(),`UPDATE database_restore_jobs SET status='running' WHERE id=? AND user_id=?`,jobID,ownerID)
	ctx,cancel:=context.WithTimeout(context.Background(),13*time.Minute);defer cancel()
	result,err:=conn.SendCommand(ctx,"restore_database_backup",req)
	if err!=nil||result==nil||!result.Ok {
		_,_=s.db.conn.ExecContext(context.Background(),`UPDATE database_restore_jobs SET status='failed',error_message='Agent restore failed' WHERE id=? AND user_id=?`,jobID,ownerID)
		_,_=s.db.conn.ExecContext(context.Background(),`UPDATE db_instances SET status='error' WHERE id=? AND user_id=?`,targetID,ownerID)
		message:="Agent database restore failed";if err!=nil{message=err.Error()}else if result!=nil&&result.Message!=""{message=result.Message}
		_ = s.db.Log(context.Background(),&shared.LogEntry{UserID:shared.PtrInt64(actorID),DeviceID:shared.PtrInt64(deviceID),Action:"database_restore_failed",Details:map[string]interface{}{"ok":false,"job_id":jobID,"backup_id":backupID,"error":message}})
		s.notifyWorkspaceEvent(context.Background(),ownerID,actorID,"error","Database restore failed",message)
		return
	}
	tx,err:=s.db.conn.BeginTx(context.Background(),nil);if err!=nil{_,_=s.db.conn.ExecContext(context.Background(),"UPDATE database_restore_jobs SET status='failed',error_message='metadata transaction failed' WHERE id=?",jobID);_,_=s.db.conn.ExecContext(context.Background(),"UPDATE db_instances SET status='error' WHERE id=?",targetID);_ = s.db.Log(context.Background(),&shared.LogEntry{UserID:shared.PtrInt64(actorID),DeviceID:shared.PtrInt64(deviceID),Action:"database_restore_failed",Details:map[string]interface{}{"ok":false,"job_id":jobID,"error":"metadata transaction failed"}})
		s.notifyWorkspaceEvent(context.Background(),ownerID,actorID,"error","Database restore failed","Control Plane could not start the metadata transaction.");return}
	_,err=tx.Exec(`INSERT INTO database_credentials(database_id,username,ciphertext,nonce,key_version) VALUES(?,?,?,?,1)`,targetID,req.Database.Username,ciphertext,nonce)
	if err==nil { _,err=tx.Exec(`UPDATE db_instances SET status='ready' WHERE id=? AND user_id=?`,targetID,ownerID) }
	if err==nil && projectID>0 && environmentID>0 { _,err=tx.Exec(`INSERT INTO project_environment_databases(project_id,environment_id,database_id) VALUES(?,?,?)`,projectID,environmentID,targetID) }
	if err==nil&&siteID!="" { _,err=tx.Exec(`INSERT INTO site_database_connections(user_id,device_id,site_id,database_id) VALUES(?,?,?,?)`,ownerID,deviceID,siteID,targetID) }
	if err==nil { _,err=tx.Exec(`UPDATE database_restore_jobs SET status='completed',error_message=NULL WHERE id=? AND user_id=?`,jobID,ownerID) }
	if err!=nil { _=tx.Rollback();_,_=s.db.conn.ExecContext(context.Background(),`UPDATE database_restore_jobs SET status='failed',error_message='Control Plane metadata save failed' WHERE id=? AND user_id=?`,jobID,ownerID);_,_=s.db.conn.ExecContext(context.Background(),`UPDATE db_instances SET status='error' WHERE id=? AND user_id=?`,targetID,ownerID);_ = s.db.Log(context.Background(),&shared.LogEntry{UserID:shared.PtrInt64(actorID),DeviceID:shared.PtrInt64(deviceID),Action:"database_restore_failed",Details:map[string]interface{}{"ok":false,"job_id":jobID,"error":"metadata save failed"}})
		s.notifyWorkspaceEvent(context.Background(),ownerID,actorID,"error","Database restore failed","Control Plane could not save restored database metadata.");return }
	if err:=tx.Commit();err!=nil { _,_=s.db.conn.ExecContext(context.Background(),`UPDATE database_restore_jobs SET status='failed',error_message='Control Plane metadata commit failed' WHERE id=? AND user_id=?`,jobID,ownerID);_,_=s.db.conn.ExecContext(context.Background(),`UPDATE db_instances SET status='error' WHERE id=? AND user_id=?`,targetID,ownerID);_ = s.db.Log(context.Background(),&shared.LogEntry{UserID:shared.PtrInt64(actorID),DeviceID:shared.PtrInt64(deviceID),Action:"database_restore_failed",Details:map[string]interface{}{"ok":false,"job_id":jobID,"error":"metadata commit failed"}})
		s.notifyWorkspaceEvent(context.Background(),ownerID,actorID,"error","Database restore failed","Control Plane could not commit restored database metadata.");return }
	_ = s.db.Log(context.Background(),&shared.LogEntry{UserID:shared.PtrInt64(actorID),DeviceID:shared.PtrInt64(deviceID),Action:"database_backup_restored",Details:map[string]interface{}{"job_id":jobID,"backup_id":backupID,"database_id":targetID}})
	s.notifyWorkspaceEvent(context.Background(),ownerID,actorID,"success","Database restore completed",fmt.Sprintf("Database #%d was restored from backup #%d.",targetID,backupID))
}
