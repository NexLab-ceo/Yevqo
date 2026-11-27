package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"veltrix/internal/shared"
)

var postgresIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}$`)

type createDatabaseInput struct {
	ProjectID int64 `json:"project_id"`
	EnvironmentID int64 `json:"environment_id"`
	ServerID int64 `json:"server_id"`
	Name string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password"`
	SiteDeviceID int64 `json:"site_device_id"`
	SiteID string `json:"site_id"`
}

func serverDataDirectory(db *sql.DB, ctx context.Context, serverID, ownerID int64) string { var path string; _ = db.QueryRowContext(ctx, `SELECT data_directory FROM database_servers WHERE id=? AND user_id=?`, serverID, ownerID).Scan(&path); return path }

func (s *Server) handleDatabaseCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet { s.handleDatabases(w,r); return }
	if r.Method != http.MethodPost { http.Error(w,"method not allowed",http.StatusMethodNotAllowed); return }
	u:=UserFromContext(r.Context())
	var in createDatabaseInput
	if err:=decodeJSON(r,&in);err!=nil { apiError(w,shared.ErrBadRequest("invalid JSON"));return }
	in.Name=strings.TrimSpace(in.Name);in.Username=strings.TrimSpace(in.Username)
	if in.ServerID<1 || !postgresIdentifier.MatchString(in.Name) || !postgresIdentifier.MatchString(in.Username) {
		apiError(w,shared.ErrBadRequest("database and username must be valid SQL identifiers (1–63 characters)"));return
	}
	if len(in.Password)<12 || len(in.Password)>256 || strings.IndexByte(in.Password,0)>=0 {
		apiError(w,shared.ErrBadRequest("password must be 12–256 characters and cannot contain NUL"));return
	}
	if in.ProjectID < 0 { apiError(w,shared.ErrBadRequest("invalid project id"));return }
	ownerID,deviceID,err:=s.authorizeDatabaseServer(r.Context(),in.ServerID,u.ID,"database.manage")
	if err!=nil { apiError(w,err);return }
	var engine,status,sslMode string;var port int
	err=s.db.conn.QueryRowContext(r.Context(),`SELECT engine,status,port,ssl_mode FROM database_servers WHERE id=? AND user_id=?`,in.ServerID,ownerID).Scan(&engine,&status,&port,&sslMode)
	if err!=nil { apiError(w,shared.ErrInternal("could not load database server"));return }
	if in.ProjectID > 0 {
		if err:=s.authorizeProjectAction(r.Context(),in.ProjectID,u.ID,"database.manage");err!=nil { apiError(w,err);return }
		var projectOwner int64
		if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT user_id FROM projects WHERE id=?`,in.ProjectID).Scan(&projectOwner);err!=nil||projectOwner!=ownerID { apiError(w,shared.ErrNotFound("project not found"));return }
		var linked int
		if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM project_database_servers WHERE project_id=? AND database_server_id=?`,in.ProjectID,in.ServerID).Scan(&linked);err!=nil||linked!=1 { apiError(w,shared.ErrNotFound("database server is not assigned to this project"));return }
	}
	if in.EnvironmentID < 0 || (in.EnvironmentID > 0 && in.ProjectID < 1) { apiError(w,shared.ErrBadRequest("environment_id requires a project_id"));return }
	if in.EnvironmentID > 0 {
		var assigned int
		if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM project_environment_database_servers WHERE project_id=? AND environment_id=? AND database_server_id=?`,in.ProjectID,in.EnvironmentID,in.ServerID).Scan(&assigned);err!=nil||assigned!=1 { apiError(w,shared.ErrNotFound("database server is not assigned to this environment"));return }
	}
	if engine!="postgresql" && engine!="mariadb" && engine!="mysql" { apiError(w,shared.ErrBadRequest("unsupported database engine"));return }
	if status!="ready" { apiError(w,shared.ErrBadRequest("database server must be ready"));return }
	if in.SiteID!="" {
		if in.SiteDeviceID==0 { in.SiteDeviceID=deviceID }
		if in.SiteDeviceID!=deviceID || len(in.SiteID)>64 { apiError(w,shared.ErrBadRequest("site and database must be on the same device for localhost connections"));return }
		var exists int
		if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM sites st JOIN devices d ON d.id=st.device_id WHERE st.device_id=? AND st.site_id=? AND d.user_id=?`,in.SiteDeviceID,in.SiteID,ownerID).Scan(&exists);err!=nil||exists!=1 { apiError(w,shared.ErrNotFound("site not found on selected device"));return }
		if err:=s.authorizeSiteProjects(r.Context(),in.SiteDeviceID,in.SiteID,ownerID,u.ID,"site.configure");err!=nil { apiError(w,err);return }
		if in.ProjectID > 0 && in.EnvironmentID > 0 {
			var siteAssigned int
			if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM project_environment_sites WHERE project_id=? AND environment_id=? AND device_id=? AND site_id=?`,in.ProjectID,in.EnvironmentID,in.SiteDeviceID,in.SiteID).Scan(&siteAssigned);err!=nil||siteAssigned!=1 {
				apiError(w,shared.ErrBadRequest("website and database must belong to the same selected project environment"));return
			}
		}
	}
	conn,online:=s.hub.Get(deviceID);if !online { apiError(w,shared.ErrBadRequest("device is offline"));return }
	ctx,cancel:=context.WithTimeout(r.Context(),2*time.Minute);defer cancel()
	var duplicate int
	if err:=s.db.conn.QueryRowContext(ctx,`SELECT COUNT(*) FROM db_instances WHERE server_id=? AND name=?`,in.ServerID,in.Name).Scan(&duplicate);err!=nil { apiError(w,shared.ErrInternal("could not validate database name"));return }
	if duplicate>0 { apiError(w,shared.ErrBadRequest("database name already exists"));return }
	res,err:=s.db.conn.ExecContext(ctx,`INSERT INTO db_instances(user_id,server_id,name,status) VALUES(?,?,?,'planned')`,ownerID,in.ServerID,in.Name)
	if err!=nil { apiError(w,shared.ErrBadRequest("database name already exists or could not be reserved"));return }
	databaseID,_:=res.LastInsertId()
	cmdResult,err:=conn.SendCommand(ctx,"create_database",&shared.CreateDatabase{Engine:engine,SSLMode:sslMode,DataDirectory:serverDataDirectory(s.db.conn,ctx,in.ServerID,ownerID),DatabaseID:databaseID,ServerID:in.ServerID,Database:in.Name,Username:in.Username,Password:in.Password,Port:uint32(port)})
	if err!=nil || !cmdResult.Ok {
		_,_=s.db.conn.ExecContext(context.Background(),`UPDATE db_instances SET status='error' WHERE id=? AND user_id=?`,databaseID,ownerID)
		if err!=nil { apiError(w,shared.ErrInternal("Agent database creation command failed"));return }
		apiError(w,shared.ErrBadRequest(cmdResult.Message));return
	}
	nonce,ciphertext,err:=encryptCredential(s.credentialKey,[]byte(in.Password))
	if err!=nil { _,_=s.db.conn.ExecContext(context.Background(),`UPDATE db_instances SET status='error' WHERE id=?`,databaseID);apiError(w,shared.ErrInternal("could not protect database credentials"));return }
	tx,err:=s.db.conn.BeginTx(ctx,nil);if err!=nil { apiError(w,shared.ErrInternal("database created but metadata transaction could not start"));return }
	_,err=tx.ExecContext(ctx,`INSERT INTO database_credentials(database_id,username,ciphertext,nonce,key_version) VALUES(?,?,?,?,1)`,databaseID,in.Username,ciphertext,nonce)
	if err==nil { _,err=tx.ExecContext(ctx,`UPDATE db_instances SET status='ready' WHERE id=? AND user_id=?`,databaseID,ownerID) }
	if err==nil && in.ProjectID>0 && in.EnvironmentID>0 {
		_,err=tx.ExecContext(ctx,`INSERT INTO project_environment_databases(project_id,environment_id,database_id) VALUES(?,?,?)`,in.ProjectID,in.EnvironmentID,databaseID)
	}
	if err==nil && in.SiteID!="" { _,err=tx.ExecContext(ctx,`INSERT INTO site_database_connections(user_id,device_id,site_id,database_id) VALUES(?,?,?,?)`,ownerID,in.SiteDeviceID,in.SiteID,databaseID) }
	if err!=nil { _=tx.Rollback();apiError(w,shared.ErrInternal("database created but credential metadata could not be saved; do not retry blindly"));return }
	if err=tx.Commit();err!=nil { apiError(w,shared.ErrInternal("database created but metadata commit failed; inspect database inventory before retrying"));return }
	_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),DeviceID:shared.PtrInt64(deviceID),Action:"database_created",Details:map[string]interface{}{"database_id":databaseID,"server_id":in.ServerID,"database":in.Name,"username":in.Username}})
	writeJSON(w,http.StatusCreated,map[string]interface{}{"ok":true,"database":map[string]interface{}{"id":databaseID,"server_id":in.ServerID,"name":in.Name,"username":in.Username,"status":"ready"},"message":"Database and least-privilege application role created. Password stored encrypted."})
}

func (s *Server) siteProjectScope(ctx context.Context, userID, deviceID int64, siteID string) (int64, error) {
	rows, err := s.db.conn.QueryContext(ctx, `SELECT ps.project_id FROM project_sites ps JOIN projects p ON p.id=ps.project_id
		WHERE p.user_id=? AND ps.device_id=? AND ps.site_id=? ORDER BY ps.project_id LIMIT 2`, userID, deviceID, siteID)
	if err != nil { return 0, err }
	defer rows.Close()
	if !rows.Next() { return 0, rows.Err() }
	var projectID int64
	if err := rows.Scan(&projectID); err != nil { return 0, err }
	if rows.Next() { return 0, shared.ErrBadRequest("site belongs to multiple projects; select the project explicitly") }
	if err := rows.Err(); err != nil { return 0, err }
	return projectID, nil
}

func (s *Server) siteEnvironmentScope(ctx context.Context, userID, deviceID int64, siteID string) (int64, int64, error) {
	// Environment scope applies to every application, not only applications
	// with a database connection: project-level ENV values must also load.
	rows, err := s.db.conn.QueryContext(ctx, `SELECT es.project_id,es.environment_id FROM project_environment_sites es
		JOIN projects p ON p.id=es.project_id WHERE p.user_id=? AND es.device_id=? AND es.site_id=?
		ORDER BY es.project_id,es.environment_id LIMIT 2`, userID, deviceID, siteID)
	if err != nil { return 0, 0, err }
	defer rows.Close()
	if !rows.Next() { return 0, 0, rows.Err() }
	var projectID, environmentID int64
	if err := rows.Scan(&projectID, &environmentID); err != nil { return 0, 0, err }
	if rows.Next() { return 0, 0, shared.ErrBadRequest("site belongs to multiple project environments; choose an environment before starting it") }
	if err := rows.Err(); err != nil { return 0, 0, err }
	return projectID, environmentID, nil
}

func siteDatabaseEnvironmentFilter(projectID, environmentID int64) (string, []interface{}, error) {
	if projectID < 0 || environmentID < 0 || (projectID == 0) != (environmentID == 0) {
		return "", nil, shared.ErrBadRequest("project_id and environment_id must be provided together")
	}
	if projectID == 0 { return "", nil, nil }
	return ` AND EXISTS (SELECT 1 FROM project_environment_databases edb
		WHERE edb.project_id=? AND edb.environment_id=? AND edb.database_id=d.id)`,
		[]interface{}{projectID, environmentID}, nil
}

func databaseConnectionDriver(engine string) string { if engine == "mariadb" || engine == "mysql" { return "mysql" }; return "pgsql" }

func (s *Server) siteDatabaseEnvironment(ctx context.Context, userID, deviceID int64, siteID string, projectID, environmentID int64) (map[string]string,error) {
	filter, filterArgs, err := siteDatabaseEnvironmentFilter(projectID, environmentID)
	if err != nil { return nil, err }
	if projectID > 0 {
		var assigned int
		err := s.db.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM project_environment_sites es
			JOIN projects p ON p.id=es.project_id
			WHERE es.project_id=? AND es.environment_id=? AND es.device_id=? AND es.site_id=? AND p.user_id=?`,
			projectID, environmentID, deviceID, siteID, userID).Scan(&assigned)
		if err != nil { return nil, err }
		if assigned != 1 { return nil, shared.ErrNotFound("site is not assigned to the selected project environment") }
	} else {
		var connections, environmentAssignments int
		if err := s.db.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM site_database_connections WHERE user_id=? AND device_id=? AND site_id=?`, userID, deviceID, siteID).Scan(&connections); err != nil { return nil, err }
		if err := s.db.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM project_environment_sites es JOIN projects p ON p.id=es.project_id WHERE p.user_id=? AND es.device_id=? AND es.site_id=?`, userID, deviceID, siteID).Scan(&environmentAssignments); err != nil { return nil, err }
		if connections > 0 && environmentAssignments > 0 {
			return nil, shared.ErrBadRequest("select the project environment before starting a site with managed database connections")
		}
	}
	query := `SELECT d.name,ds.port,ds.ssl_mode,dc.username,dc.ciphertext,dc.nonce,ds.engine,ds.data_directory
		FROM site_database_connections c
		JOIN db_instances d ON d.id=c.database_id
		JOIN database_servers ds ON ds.id=d.server_id AND ds.user_id=d.user_id
		JOIN database_credentials dc ON dc.database_id=d.id
		WHERE c.user_id=? AND c.device_id=? AND c.site_id=? AND d.status='ready' AND ds.status='ready'` + filter + ` ORDER BY c.id`
	args := []interface{}{userID, deviceID, siteID}
	args = append(args, filterArgs...)
	rows,err:=s.db.conn.QueryContext(ctx,query,args...)
	if err!=nil{return nil,err};defer rows.Close()
	type databaseEnv struct { Name string `json:"name"`; Engine string `json:"engine"`; DataDirectory string `json:"data_directory"`; Host string `json:"host"`; Port int `json:"port"`; SSLMode string `json:"ssl_mode"`; Username string `json:"username"`; Password string `json:"password"` }
	all:=make([]databaseEnv,0)
	for rows.Next(){
		var item databaseEnv;var ciphertext,nonce []byte
		if err:=rows.Scan(&item.Name,&item.Port,&item.SSLMode,&item.Username,&ciphertext,&nonce,&item.Engine,&item.DataDirectory);err!=nil{return nil,err}
		plain,err:=decryptCredential(s.credentialKey,nonce,ciphertext);if err!=nil{return nil,err}
		item.Host="127.0.0.1";item.Password=string(plain);all=append(all,item)
	}
	if err:=rows.Err();err!=nil{return nil,err}
	if len(all)==0{return nil,nil}
	encoded,err:=json.Marshal(all);if err!=nil{return nil,err}
	first:=all[0]
	env := map[string]string{"DB_CONNECTION":databaseConnectionDriver(first.Engine),"DB_HOST":first.Host,"DB_PORT":strconv.Itoa(first.Port),"DB_SSLMODE":first.SSLMode,"DB_DATABASE":first.Name,"DB_USERNAME":first.Username,"DB_PASSWORD":first.Password,"VELTRIX_DATABASES":string(encoded)}
	if first.Engine == "mariadb" || first.Engine == "mysql" { env["DB_SSL_CA"] = filepath.Join(first.DataDirectory, "server.crt") }
	return env,nil
}

func (s *Server) handleDatabaseConnections(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	switch r.Method {
	case http.MethodGet:
		if databaseID, _ := strconv.ParseInt(r.URL.Query().Get("database_id"), 10, 64); databaseID > 0 {
			ownerID, _, _, accessErr := s.authorizeDatabaseInstance(r.Context(), databaseID, u.ID, "database.read")
			if accessErr != nil { apiError(w, accessErr); return }
			rows, err := s.db.conn.QueryContext(r.Context(), `SELECT c.id,c.device_id,c.site_id,d.name,dc.username,ds.port,ds.engine
				FROM site_database_connections c JOIN db_instances d ON d.id=c.database_id
				JOIN database_servers ds ON ds.id=d.server_id JOIN database_credentials dc ON dc.database_id=d.id
				WHERE c.user_id=? AND c.database_id=? ORDER BY c.id`, ownerID, databaseID)
			if err != nil { apiError(w, shared.ErrInternal("could not load database links")); return }
			defer rows.Close()
			out := make([]map[string]interface{}, 0)
			for rows.Next() {
				var id, deviceID int64
				var siteID, name, username, engine string
				var port int
				if err := rows.Scan(&id, &deviceID, &siteID, &name, &username, &port, &engine); err != nil { apiError(w, shared.ErrInternal("could not read database links")); return }
				if err := s.authorizeSiteProjects(r.Context(), deviceID, siteID, ownerID, u.ID, "device.read"); err != nil {
					var apiErr *shared.APIError
					if errors.As(err, &apiErr) && apiErr.Code == http.StatusForbidden { continue }
					apiError(w, err); return
				}
				out = append(out, map[string]interface{}{"id": id, "device_id": deviceID, "site_id": siteID, "database": name, "engine": engine, "username": username, "host": "127.0.0.1", "port": port})
			}
			if err := rows.Err(); err != nil { apiError(w, shared.ErrInternal("could not read database links")); return }
			writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "connections": out})
			return
		}
		deviceID, _ := strconv.ParseInt(r.URL.Query().Get("device_id"), 10, 64)
		siteID := r.URL.Query().Get("site_id")
		if deviceID < 1 || siteID == "" { apiError(w, shared.ErrBadRequest("device_id and site_id are required")); return }
		var ownerID int64
		if err := s.db.conn.QueryRowContext(r.Context(), "SELECT user_id FROM devices WHERE id=?", deviceID).Scan(&ownerID); err != nil {
			apiError(w, shared.ErrNotFound("device not found")); return
		}
		if err := s.authorizeSiteProjects(r.Context(), deviceID, siteID, ownerID, u.ID, "device.read"); err != nil { apiError(w, err); return }
		rows, err := s.db.conn.QueryContext(r.Context(), `SELECT c.id,c.database_id,d.name,ds.port,dc.username
			FROM site_database_connections c JOIN db_instances d ON d.id=c.database_id
			JOIN database_servers ds ON ds.id=d.server_id JOIN database_credentials dc ON dc.database_id=d.id
			WHERE c.user_id=? AND c.device_id=? AND c.site_id=? ORDER BY c.id`, ownerID, deviceID, siteID)
		if err != nil { apiError(w, shared.ErrInternal("could not load site database connections")); return }
		defer rows.Close()
		out := make([]map[string]interface{}, 0)
		for rows.Next() {
			var id, databaseID int64
			var name, username string
			var port int
			if err := rows.Scan(&id, &databaseID, &name, &port, &username); err != nil { apiError(w, shared.ErrInternal("could not read connections")); return }
			if _, _, _, accessErr := s.authorizeDatabaseInstance(r.Context(), databaseID, u.ID, "database.read"); accessErr != nil {
				var apiErr *shared.APIError
				if errors.As(accessErr, &apiErr) && apiErr.Code == http.StatusForbidden { continue }
				apiError(w, accessErr); return
			}
			out = append(out, map[string]interface{}{"id": id, "database_id": databaseID, "database": name, "username": username, "host": "127.0.0.1", "port": port})
		}
		if err := rows.Err(); err != nil { apiError(w, shared.ErrInternal("could not read connections")); return }
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "connections": out})
	case http.MethodPost:
		var in struct { DeviceID int64 `json:"device_id"`; SiteID string `json:"site_id"`; DatabaseID int64 `json:"database_id"` }
		if err := decodeJSON(r, &in); err != nil || in.DeviceID < 1 || in.DatabaseID < 1 || !validBackupSiteID(in.SiteID) {
			apiError(w, shared.ErrBadRequest("invalid database connection request")); return
		}
		ownerID, _, dbDeviceID, accessErr := s.authorizeDatabaseInstance(r.Context(), in.DatabaseID, u.ID, "database.manage")
		if accessErr != nil { apiError(w, accessErr); return }
		if dbDeviceID != in.DeviceID { apiError(w, shared.ErrBadRequest("website and database must be on the same device while database access is localhost-only")); return }
		siteOwnerID, siteErr := s.authorizeDeviceOwner(r.Context(), in.DeviceID, u.ID, "device.read")
		if siteErr != nil { apiError(w, siteErr); return }
		if siteOwnerID != ownerID { apiError(w, shared.ErrNotFound("website not found")); return }
		if err := s.authorizeSiteProjects(r.Context(), in.DeviceID, in.SiteID, ownerID, u.ID, "site.configure"); err != nil { apiError(w, err); return }
		var dbStatus, serverStatus string
		if err := s.db.conn.QueryRowContext(r.Context(), `SELECT d.status,ds.status FROM db_instances d JOIN database_servers ds ON ds.id=d.server_id WHERE d.id=? AND d.user_id=?`, in.DatabaseID, ownerID).Scan(&dbStatus, &serverStatus); err != nil {
			apiError(w, shared.ErrInternal("could not inspect database")); return
		}
		if dbStatus != "ready" || serverStatus != "ready" { apiError(w, shared.ErrBadRequest("database and server must be ready")); return }
		var siteCount int
		if err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM sites st JOIN devices d ON d.id=st.device_id WHERE st.device_id=? AND st.site_id=? AND d.user_id=?`, in.DeviceID, in.SiteID, ownerID).Scan(&siteCount); err != nil || siteCount != 1 {
			apiError(w, shared.ErrNotFound("website not found")); return
		}
		res, err := s.db.conn.ExecContext(r.Context(), `INSERT INTO site_database_connections(user_id,device_id,site_id,database_id) VALUES(?,?,?,?)`, ownerID, in.DeviceID, in.SiteID, in.DatabaseID)
		if err != nil { apiError(w, shared.ErrBadRequest("this database is already linked to the website or the link could not be created")); return }
		id, _ := res.LastInsertId()
		writeJSON(w, http.StatusCreated, map[string]interface{}{"ok": true, "connection": map[string]interface{}{"id": id, "device_id": in.DeviceID, "site_id": in.SiteID, "database_id": in.DatabaseID}})
	case http.MethodDelete:
		var in struct { ID int64 `json:"id"` }
		if err := decodeJSON(r, &in); err != nil || in.ID < 1 { apiError(w, shared.ErrBadRequest("invalid connection id")); return }
		var ownerID, deviceID, databaseID int64
		var siteID string
		err := s.db.conn.QueryRowContext(r.Context(), `SELECT user_id,device_id,site_id,database_id FROM site_database_connections WHERE id=?`, in.ID).Scan(&ownerID, &deviceID, &siteID, &databaseID)
		if err == sql.ErrNoRows { apiError(w, shared.ErrNotFound("connection not found")); return }
		if err != nil { apiError(w, shared.ErrInternal("could not resolve database connection owner")); return }
		databaseOwnerID, _, databaseDeviceID, accessErr := s.authorizeDatabaseInstance(r.Context(), databaseID, u.ID, "database.manage")
		if accessErr != nil { apiError(w, accessErr); return }
		if databaseOwnerID != ownerID || databaseDeviceID != deviceID { apiError(w, shared.ErrNotFound("connection not found")); return }
		var siteOwnerID int64
		if err := s.db.conn.QueryRowContext(r.Context(), "SELECT user_id FROM devices WHERE id=?", deviceID).Scan(&siteOwnerID); err != nil || siteOwnerID != ownerID { apiError(w, shared.ErrNotFound("connection not found")); return }
		if err := s.authorizeSiteProjects(r.Context(), deviceID, siteID, ownerID, u.ID, "site.configure"); err != nil { apiError(w, err); return }
		res, err := s.db.conn.ExecContext(r.Context(), `DELETE FROM site_database_connections WHERE id=? AND user_id=?`, in.ID, ownerID)
		if err != nil { apiError(w, shared.ErrInternal("could not remove site connection")); return }
		n, _ := res.RowsAffected()
		if n == 0 { apiError(w, shared.ErrNotFound("connection not found")); return }
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// Avoid accidental disclosure: credentials are only used by internal provisioning code, never serialized.
func redactDatabasePayload(b []byte) []byte { var v map[string]interface{};if json.Unmarshal(b,&v)!=nil{return nil};delete(v,"password");out,_:=json.Marshal(v);return out }

func (s *Server) handleDatabaseByID(w http.ResponseWriter,r *http.Request) {
	if r.Method!=http.MethodGet { http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return }
	u:=UserFromContext(r.Context())
	raw:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/databases/"),"/")
	id,err:=strconv.ParseInt(raw,10,64)
	if err!=nil||id<1 { apiError(w,shared.ErrBadRequest("invalid database id"));return }
	ownerID,_,_,accessErr:=s.authorizeDatabaseInstance(r.Context(),id,u.ID,"database.read");if accessErr!=nil{apiError(w,accessErr);return}
	var databaseID,serverID,deviceID int64
	var name,status,serverName,engine,sslMode string
	var username sql.NullString
	var restoreStatus sql.NullString
	var port int
	var created time.Time
	err=s.db.conn.QueryRowContext(r.Context(),`SELECT d.id,d.server_id,d.name,d.status,d.created_at,ds.device_id,ds.name,ds.engine,ds.port,ds.ssl_mode,dc.username,(SELECT status FROM database_restore_jobs WHERE target_database_id=d.id ORDER BY id DESC LIMIT 1)
		FROM db_instances d JOIN database_servers ds ON ds.id=d.server_id
		LEFT JOIN database_credentials dc ON dc.database_id=d.id
		WHERE d.id=? AND d.user_id=?`,id,ownerID).Scan(&databaseID,&serverID,&name,&status,&created,&deviceID,&serverName,&engine,&port,&sslMode,&username,&restoreStatus)
	if err==sql.ErrNoRows { apiError(w,shared.ErrNotFound("database not found"));return }
	if err!=nil { apiError(w,shared.ErrInternal("could not load database details"));return }
	var linkedSites int
	if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM site_database_connections WHERE database_id=? AND user_id=?`,id,ownerID).Scan(&linkedSites);err!=nil { apiError(w,shared.ErrInternal("could not load database connections"));return }
	user:=""
	if username.Valid { user=username.String }
	permissions := []string{"CONNECT","USAGE ON SCHEMA public","SELECT","INSERT","UPDATE","DELETE","USAGE/SELECT/UPDATE ON SEQUENCES"}
	if engine == "mariadb" || engine == "mysql" { permissions = []string{"SELECT","INSERT","UPDATE","DELETE","CREATE","DROP","INDEX","ALTER","REFERENCES","EXECUTE","TRIGGER"} }
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"database":map[string]interface{}{
		"id":databaseID,"server_id":serverID,"name":name,"status":status,"created_at":created,
		"device_id":deviceID,"server_name":serverName,"engine":engine,"port":port,
		"username":user,"host":"127.0.0.1","ssl_mode":sslMode,"restore_status":restoreStatus.String,"linked_sites":linkedSites,
		"role_type":"Application user","permissions":permissions,
	}})
}

func (s *Server) handleDatabaseConnectionTest(w http.ResponseWriter,r *http.Request) {
	if r.Method!=http.MethodPost { http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return }
	u:=UserFromContext(r.Context())
	raw:=strings.TrimSuffix(strings.TrimPrefix(r.URL.Path,"/api/databases/"),"/test")
	id,err:=strconv.ParseInt(strings.Trim(raw,"/"),10,64);if err!=nil||id<1{apiError(w,shared.ErrBadRequest("invalid database id"));return}
	ownerID,_,_,accessErr:=s.authorizeDatabaseInstance(r.Context(),id,u.ID,"database.read");if accessErr!=nil{apiError(w,accessErr);return}
	var name,serverStatus,databaseStatus,sslMode,engine,dataDir string;var deviceID int64;var port int;var username sql.NullString;var ciphertext,nonce []byte
	err=s.db.conn.QueryRowContext(r.Context(),`SELECT d.name,ds.device_id,ds.port,ds.ssl_mode,ds.status,d.status,dc.username,dc.ciphertext,dc.nonce,ds.engine,ds.data_directory
		FROM db_instances d JOIN database_servers ds ON ds.id=d.server_id
		JOIN database_credentials dc ON dc.database_id=d.id
		WHERE d.id=? AND d.user_id=?`,id,ownerID).Scan(&name,&deviceID,&port,&sslMode,&serverStatus,&databaseStatus,&username,&ciphertext,&nonce,&engine,&dataDir)
	if err==sql.ErrNoRows{apiError(w,shared.ErrNotFound("database credentials not found"));return}
	if err!=nil{apiError(w,shared.ErrInternal("could not load database connection"));return}
	if databaseStatus!="ready"||serverStatus!="ready"||!username.Valid{apiError(w,shared.ErrBadRequest("database and server must be ready"));return}
	password,err:=decryptCredential(s.credentialKey,nonce,ciphertext);if err!=nil{apiError(w,shared.ErrInternal("could not decrypt database credentials"));return}
	conn,online:=s.hub.Get(deviceID);if !online{apiError(w,shared.ErrBadRequest("device is offline"));return}
	ctx,cancel:=context.WithTimeout(r.Context(),20*time.Second);defer cancel()
	result,err:=conn.SendCommand(ctx,"test_database_connection",&shared.TestDatabaseConnection{Engine:engine,DataDirectory:dataDir,DatabaseID:id,Database:name,Username:username.String,Password:string(password),Port:uint32(port),SSLMode:sslMode})
	if err!=nil{apiError(w,shared.ErrInternal("database connection test command failed"));return}
	if !result.Ok{apiError(w,shared.ErrBadRequest(result.Message));return}
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"message":engine+" connection test succeeded"})
}
