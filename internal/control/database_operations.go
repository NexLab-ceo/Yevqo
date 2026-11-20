package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"veltrix/internal/shared"
)

var postgresIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}$`)

type createDatabaseInput struct {
	ProjectID int64 `json:"project_id"`
	ServerID int64 `json:"server_id"`
	Name string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password"`
	SiteDeviceID int64 `json:"site_device_id"`
	SiteID string `json:"site_id"`
}

func (s *Server) handleDatabaseCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet { s.handleDatabases(w,r); return }
	if r.Method != http.MethodPost { http.Error(w,"method not allowed",http.StatusMethodNotAllowed); return }
	u:=UserFromContext(r.Context())
	var in createDatabaseInput
	if err:=decodeJSON(r,&in);err!=nil { apiError(w,shared.ErrBadRequest("invalid JSON"));return }
	in.Name=strings.TrimSpace(in.Name);in.Username=strings.TrimSpace(in.Username)
	if in.ServerID<1 || !postgresIdentifier.MatchString(in.Name) || !postgresIdentifier.MatchString(in.Username) {
		apiError(w,shared.ErrBadRequest("database and username must be valid PostgreSQL identifiers (1–63 characters)"));return
	}
	if len(in.Password)<12 || len(in.Password)>256 || strings.IndexByte(in.Password,0)>=0 {
		apiError(w,shared.ErrBadRequest("password must be 12–256 characters and cannot contain NUL"));return
	}
	if in.ProjectID < 0 { apiError(w,shared.ErrBadRequest("invalid project id"));return }
	ownerID,deviceID,err:=s.authorizeDatabaseServer(r.Context(),in.ServerID,u.ID,"database.manage")
	if err!=nil { apiError(w,err);return }
	var engine,status string;var port int
	err=s.db.conn.QueryRowContext(r.Context(),`SELECT engine,status,port FROM database_servers WHERE id=? AND user_id=?`,in.ServerID,ownerID).Scan(&engine,&status,&port)
	if err!=nil { apiError(w,shared.ErrInternal("could not load database server"));return }
	if in.ProjectID > 0 {
		if err:=s.authorizeProjectAction(r.Context(),in.ProjectID,u.ID,"database.manage");err!=nil { apiError(w,err);return }
		var projectOwner int64
		if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT user_id FROM projects WHERE id=?`,in.ProjectID).Scan(&projectOwner);err!=nil||projectOwner!=ownerID { apiError(w,shared.ErrNotFound("project not found"));return }
		var linked int
		if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM project_database_servers WHERE project_id=? AND database_server_id=?`,in.ProjectID,in.ServerID).Scan(&linked);err!=nil||linked!=1 { apiError(w,shared.ErrNotFound("database server is not assigned to this project"));return }
	}
	if engine!="postgresql" { apiError(w,shared.ErrBadRequest("database creation currently supports PostgreSQL only"));return }
	if status!="ready" { apiError(w,shared.ErrBadRequest("database server must be ready"));return }
	if in.SiteID!="" {
		if in.SiteDeviceID==0 { in.SiteDeviceID=deviceID }
		if in.SiteDeviceID!=deviceID || len(in.SiteID)>64 { apiError(w,shared.ErrBadRequest("site and database must be on the same device for localhost connections"));return }
		var exists int
		if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM sites st JOIN devices d ON d.id=st.device_id WHERE st.device_id=? AND st.site_id=? AND d.user_id=?`,in.SiteDeviceID,in.SiteID,ownerID).Scan(&exists);err!=nil||exists!=1 { apiError(w,shared.ErrNotFound("site not found on selected device"));return }
	}
	conn,online:=s.hub.Get(deviceID);if !online { apiError(w,shared.ErrBadRequest("device is offline"));return }
	ctx,cancel:=context.WithTimeout(r.Context(),2*time.Minute);defer cancel()
	var duplicate int
	if err:=s.db.conn.QueryRowContext(ctx,`SELECT COUNT(*) FROM db_instances WHERE server_id=? AND name=?`,in.ServerID,in.Name).Scan(&duplicate);err!=nil { apiError(w,shared.ErrInternal("could not validate database name"));return }
	if duplicate>0 { apiError(w,shared.ErrBadRequest("database name already exists"));return }
	res,err:=s.db.conn.ExecContext(ctx,`INSERT INTO db_instances(user_id,server_id,name,status) VALUES(?,?,?,'planned')`,ownerID,in.ServerID,in.Name)
	if err!=nil { apiError(w,shared.ErrBadRequest("database name already exists or could not be reserved"));return }
	databaseID,_:=res.LastInsertId()
	cmdResult,err:=conn.SendCommand(ctx,"create_database",&shared.CreateDatabase{DatabaseID:databaseID,ServerID:in.ServerID,Database:in.Name,Username:in.Username,Password:in.Password,Port:uint32(port)})
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
	if err==nil && in.SiteID!="" { _,err=tx.ExecContext(ctx,`INSERT INTO site_database_connections(user_id,device_id,site_id,database_id) VALUES(?,?,?,?)`,ownerID,in.SiteDeviceID,in.SiteID,databaseID) }
	if err!=nil { _=tx.Rollback();apiError(w,shared.ErrInternal("database created but credential metadata could not be saved; do not retry blindly"));return }
	if err=tx.Commit();err!=nil { apiError(w,shared.ErrInternal("database created but metadata commit failed; inspect database inventory before retrying"));return }
	_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),DeviceID:shared.PtrInt64(deviceID),Action:"database_created",Details:map[string]interface{}{"database_id":databaseID,"server_id":in.ServerID,"database":in.Name,"username":in.Username}})
	writeJSON(w,http.StatusCreated,map[string]interface{}{"ok":true,"database":map[string]interface{}{"id":databaseID,"server_id":in.ServerID,"name":in.Name,"username":in.Username,"status":"ready"},"message":"Database and least-privilege application role created. Password stored encrypted."})
}

func (s *Server) siteDatabaseEnvironment(ctx context.Context, userID, deviceID int64, siteID string) (map[string]string,error) {
	rows,err:=s.db.conn.QueryContext(ctx,`SELECT d.name,ds.port,ds.ssl_mode,dc.username,dc.ciphertext,dc.nonce
		FROM site_database_connections c
		JOIN db_instances d ON d.id=c.database_id
		JOIN database_servers ds ON ds.id=d.server_id
		JOIN database_credentials dc ON dc.database_id=d.id
		WHERE c.user_id=? AND c.device_id=? AND c.site_id=? AND d.status='ready' AND ds.status='ready'
		ORDER BY c.id`,userID,deviceID,siteID)
	if err!=nil{return nil,err};defer rows.Close()
	type databaseEnv struct { Name string `json:"name"`; Host string `json:"host"`; Port int `json:"port"`; SSLMode string `json:"ssl_mode"`; Username string `json:"username"`; Password string `json:"password"` }
	all:=make([]databaseEnv,0)
	for rows.Next(){
		var item databaseEnv;var ciphertext,nonce []byte
		if err:=rows.Scan(&item.Name,&item.Port,&item.SSLMode,&item.Username,&ciphertext,&nonce);err!=nil{return nil,err}
		plain,err:=decryptCredential(s.credentialKey,nonce,ciphertext);if err!=nil{return nil,err}
		item.Host="127.0.0.1";item.Password=string(plain);all=append(all,item)
	}
	if err:=rows.Err();err!=nil{return nil,err}
	if len(all)==0{return nil,nil}
	encoded,err:=json.Marshal(all);if err!=nil{return nil,err}
	first:=all[0]
	return map[string]string{"DB_CONNECTION":"pgsql","DB_HOST":first.Host,"DB_PORT":strconv.Itoa(first.Port),"DB_SSLMODE":first.SSLMode,"DB_DATABASE":first.Name,"DB_USERNAME":first.Username,"DB_PASSWORD":first.Password,"VELTRIX_DATABASES":string(encoded)},nil
}

func (s *Server) handleDatabaseConnections(w http.ResponseWriter,r *http.Request) {
	u:=UserFromContext(r.Context())
	if r.Method==http.MethodGet {
		if databaseID,_:=strconv.ParseInt(r.URL.Query().Get("database_id"),10,64);databaseID>0 {
			rows,err:=s.db.conn.QueryContext(r.Context(),`SELECT c.id,c.device_id,c.site_id,d.name,dc.username,ds.port
				FROM site_database_connections c JOIN db_instances d ON d.id=c.database_id
				JOIN database_servers ds ON ds.id=d.server_id JOIN database_credentials dc ON dc.database_id=d.id
				WHERE c.user_id=? AND c.database_id=? ORDER BY c.id`,u.ID,databaseID)
			if err!=nil { apiError(w,shared.ErrInternal("could not load database links"));return };defer rows.Close()
			out:=make([]map[string]interface{},0)
			for rows.Next(){var id,deviceID int64;var siteID,name,username string;var port int;if err:=rows.Scan(&id,&deviceID,&siteID,&name,&username,&port);err!=nil{apiError(w,shared.ErrInternal("could not read database links"));return};out=append(out,map[string]interface{}{"id":id,"device_id":deviceID,"site_id":siteID,"database":name,"username":username,"host":"127.0.0.1","port":port})}
			writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"connections":out});return
		}
		deviceID,_:=strconv.ParseInt(r.URL.Query().Get("device_id"),10,64)
		siteID:=r.URL.Query().Get("site_id")
		if deviceID<1 || siteID=="" { apiError(w,shared.ErrBadRequest("device_id and site_id are required"));return }
		rows,err:=s.db.conn.QueryContext(r.Context(),`SELECT c.id,c.database_id,d.name,ds.port,dc.username FROM site_database_connections c JOIN db_instances d ON d.id=c.database_id JOIN database_servers ds ON ds.id=d.server_id JOIN database_credentials dc ON dc.database_id=d.id WHERE c.user_id=? AND c.device_id=? AND c.site_id=? ORDER BY c.id`,u.ID,deviceID,siteID)
		if err!=nil { apiError(w,shared.ErrInternal("could not load site database connections"));return };defer rows.Close()
		out:=make([]map[string]interface{},0)
		for rows.Next(){var id,databaseID int64;var name,username string;var port int;if err:=rows.Scan(&id,&databaseID,&name,&port,&username);err!=nil{apiError(w,shared.ErrInternal("could not read connections"));return};out=append(out,map[string]interface{}{"id":id,"database_id":databaseID,"database":name,"username":username,"host":"127.0.0.1","port":port})}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"connections":out});return
	}
	if r.Method==http.MethodPost {
		var in struct{DeviceID int64 `json:"device_id"`;SiteID string `json:"site_id"`;DatabaseID int64 `json:"database_id"`}
		if err:=decodeJSON(r,&in);err!=nil||in.DeviceID<1||in.DatabaseID<1||!validBackupSiteID(in.SiteID){apiError(w,shared.ErrBadRequest("invalid database connection request"));return}
		var dbDeviceID int64;var dbStatus,serverStatus string
		err:=s.db.conn.QueryRowContext(r.Context(),`SELECT ds.device_id,d.status,ds.status FROM db_instances d JOIN database_servers ds ON ds.id=d.server_id WHERE d.id=? AND d.user_id=?`,in.DatabaseID,u.ID).Scan(&dbDeviceID,&dbStatus,&serverStatus)
		if err==sql.ErrNoRows{apiError(w,shared.ErrNotFound("database not found"));return};if err!=nil{apiError(w,shared.ErrInternal("could not inspect database"));return}
		if dbDeviceID!=in.DeviceID{apiError(w,shared.ErrBadRequest("website and database must be on the same device while database access is localhost-only"));return}
		if dbStatus!="ready"||serverStatus!="ready"{apiError(w,shared.ErrBadRequest("database and server must be ready"));return}
		var siteCount int;if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM sites st JOIN devices d ON d.id=st.device_id WHERE st.device_id=? AND st.site_id=? AND d.user_id=?`,in.DeviceID,in.SiteID,u.ID).Scan(&siteCount);err!=nil||siteCount!=1{apiError(w,shared.ErrNotFound("website not found"));return}
		res,err:=s.db.conn.ExecContext(r.Context(),`INSERT INTO site_database_connections(user_id,device_id,site_id,database_id) VALUES(?,?,?,?)`,u.ID,in.DeviceID,in.SiteID,in.DatabaseID)
		if err!=nil{apiError(w,shared.ErrBadRequest("this database is already linked to the website or the link could not be created"));return}
		id,_:=res.LastInsertId();writeJSON(w,http.StatusCreated,map[string]interface{}{"ok":true,"connection":map[string]interface{}{"id":id,"device_id":in.DeviceID,"site_id":in.SiteID,"database_id":in.DatabaseID}});return
	}
	if r.Method!=http.MethodDelete { http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return }
	var in struct{ID int64 `json:"id"`}
	if err:=decodeJSON(r,&in);err!=nil||in.ID<1 { apiError(w,shared.ErrBadRequest("invalid connection id"));return }
	res,err:=s.db.conn.ExecContext(r.Context(),`DELETE FROM site_database_connections WHERE id=? AND user_id=?`,in.ID,u.ID)
	if err!=nil { apiError(w,shared.ErrInternal("could not remove site connection"));return }
	n,_:=res.RowsAffected();if n==0{apiError(w,shared.ErrNotFound("connection not found"));return}
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true})
}

// Avoid accidental disclosure: credentials are only used by internal provisioning code, never serialized.
func redactDatabasePayload(b []byte) []byte { var v map[string]interface{};if json.Unmarshal(b,&v)!=nil{return nil};delete(v,"password");out,_:=json.Marshal(v);return out }

func (s *Server) handleDatabaseByID(w http.ResponseWriter,r *http.Request) {
	if r.Method!=http.MethodGet { http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return }
	u:=UserFromContext(r.Context())
	raw:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/databases/"),"/")
	id,err:=strconv.ParseInt(raw,10,64)
	if err!=nil||id<1 { apiError(w,shared.ErrBadRequest("invalid database id"));return }
	ownerID,_,_,accessErr:=s.authorizeDatabaseInstance(r.Context(),id,ownerID,"database.read");if accessErr!=nil{apiError(w,accessErr);return}
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
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"database":map[string]interface{}{
		"id":databaseID,"server_id":serverID,"name":name,"status":status,"created_at":created,
		"device_id":deviceID,"server_name":serverName,"engine":engine,"port":port,
		"username":user,"host":"127.0.0.1","ssl_mode":sslMode,"restore_status":restoreStatus.String,"linked_sites":linkedSites,
		"role_type":"Application user","permissions":[]string{"CONNECT","USAGE ON SCHEMA public","SELECT","INSERT","UPDATE","DELETE","USAGE/SELECT/UPDATE ON SEQUENCES"},
	}})
}

func (s *Server) handleDatabaseConnectionTest(w http.ResponseWriter,r *http.Request) {
	if r.Method!=http.MethodPost { http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return }
	u:=UserFromContext(r.Context())
	raw:=strings.TrimSuffix(strings.TrimPrefix(r.URL.Path,"/api/databases/"),"/test")
	id,err:=strconv.ParseInt(strings.Trim(raw,"/"),10,64);if err!=nil||id<1{apiError(w,shared.ErrBadRequest("invalid database id"));return}
	ownerID,_,_,accessErr:=s.authorizeDatabaseInstance(r.Context(),id,ownerID,"database.read");if accessErr!=nil{apiError(w,accessErr);return}
	var name,serverStatus,databaseStatus,sslMode string;var deviceID int64;var port int;var username sql.NullString;var ciphertext,nonce []byte
	err=s.db.conn.QueryRowContext(r.Context(),`SELECT d.name,ds.device_id,ds.port,ds.ssl_mode,ds.status,d.status,dc.username,dc.ciphertext,dc.nonce
		FROM db_instances d JOIN database_servers ds ON ds.id=d.server_id
		JOIN database_credentials dc ON dc.database_id=d.id
		WHERE d.id=? AND d.user_id=?`,id,ownerID).Scan(&name,&deviceID,&port,&sslMode,&serverStatus,&databaseStatus,&username,&ciphertext,&nonce)
	if err==sql.ErrNoRows{apiError(w,shared.ErrNotFound("database credentials not found"));return}
	if err!=nil{apiError(w,shared.ErrInternal("could not load database connection"));return}
	if databaseStatus!="ready"||serverStatus!="ready"||!username.Valid{apiError(w,shared.ErrBadRequest("database and server must be ready"));return}
	password,err:=decryptCredential(s.credentialKey,nonce,ciphertext);if err!=nil{apiError(w,shared.ErrInternal("could not decrypt database credentials"));return}
	conn,online:=s.hub.Get(deviceID);if !online{apiError(w,shared.ErrBadRequest("device is offline"));return}
	ctx,cancel:=context.WithTimeout(r.Context(),20*time.Second);defer cancel()
	result,err:=conn.SendCommand(ctx,"test_database_connection",&shared.TestDatabaseConnection{DatabaseID:id,Database:name,Username:username.String,Password:string(password),Port:uint32(port),SSLMode:sslMode})
	if err!=nil{apiError(w,shared.ErrInternal("database connection test command failed"));return}
	if !result.Ok{apiError(w,shared.ErrBadRequest(result.Message));return}
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"message":"PostgreSQL connection test succeeded"})
}
