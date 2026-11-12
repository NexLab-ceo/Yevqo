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
	var deviceID int64;var engine,status string;var port int
	err:=s.db.conn.QueryRowContext(r.Context(),`SELECT device_id,engine,status,port FROM database_servers WHERE id=? AND user_id=?`,in.ServerID,u.ID).Scan(&deviceID,&engine,&status,&port)
	if err==sql.ErrNoRows { apiError(w,shared.ErrNotFound("database server not found"));return }
	if err!=nil { apiError(w,shared.ErrInternal("could not load database server"));return }
	if engine!="postgresql" { apiError(w,shared.ErrBadRequest("database creation currently supports PostgreSQL only"));return }
	if status!="ready" { apiError(w,shared.ErrBadRequest("database server must be ready"));return }
	if in.SiteID!="" {
		if in.SiteDeviceID==0 { in.SiteDeviceID=deviceID }
		if in.SiteDeviceID!=deviceID || len(in.SiteID)>64 { apiError(w,shared.ErrBadRequest("site and database must be on the same device for localhost connections"));return }
		var exists int
		if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM sites WHERE device_id=? AND site_id=?`,in.SiteDeviceID,in.SiteID).Scan(&exists);err!=nil||exists!=1 { apiError(w,shared.ErrNotFound("site not found on selected device"));return }
	}
	conn,online:=s.hub.Get(deviceID);if !online { apiError(w,shared.ErrBadRequest("device is offline"));return }
	ctx,cancel:=context.WithTimeout(r.Context(),2*time.Minute);defer cancel()
	var duplicate int
	if err:=s.db.conn.QueryRowContext(ctx,`SELECT COUNT(*) FROM databases WHERE server_id=? AND name=?`,in.ServerID,in.Name).Scan(&duplicate);err!=nil { apiError(w,shared.ErrInternal("could not validate database name"));return }
	if duplicate>0 { apiError(w,shared.ErrBadRequest("database name already exists"));return }
	res,err:=s.db.conn.ExecContext(ctx,`INSERT INTO databases(user_id,server_id,name,status) VALUES(?,?,?,'planned')`,u.ID,in.ServerID,in.Name)
	if err!=nil { apiError(w,shared.ErrBadRequest("database name already exists or could not be reserved"));return }
	databaseID,_:=res.LastInsertId()
	cmdResult,err:=conn.SendCommand(ctx,"create_database",&shared.CreateDatabase{DatabaseID:databaseID,ServerID:in.ServerID,Database:in.Name,Username:in.Username,Password:in.Password,Port:uint32(port)})
	if err!=nil || !cmdResult.Ok {
		_,_=s.db.conn.ExecContext(context.Background(),`UPDATE databases SET status='error' WHERE id=? AND user_id=?`,databaseID,u.ID)
		if err!=nil { apiError(w,shared.ErrInternal("Agent database creation command failed"));return }
		apiError(w,shared.ErrBadRequest(cmdResult.Message));return
	}
	nonce,ciphertext,err:=encryptCredential(s.credentialKey,[]byte(in.Password))
	if err!=nil { _,_=s.db.conn.ExecContext(context.Background(),`UPDATE databases SET status='error' WHERE id=?`,databaseID);apiError(w,shared.ErrInternal("could not protect database credentials"));return }
	tx,err:=s.db.conn.BeginTx(ctx,nil);if err!=nil { apiError(w,shared.ErrInternal("database created but metadata transaction could not start"));return }
	_,err=tx.ExecContext(ctx,`INSERT INTO database_credentials(database_id,username,ciphertext,nonce,key_version) VALUES(?,?,?,?,1)`,databaseID,in.Username,ciphertext,nonce)
	if err==nil { _,err=tx.ExecContext(ctx,`UPDATE databases SET status='ready' WHERE id=? AND user_id=?`,databaseID,u.ID) }
	if err==nil && in.SiteID!="" { _,err=tx.ExecContext(ctx,`INSERT INTO site_database_connections(user_id,device_id,site_id,database_id) VALUES(?,?,?,?)`,u.ID,in.SiteDeviceID,in.SiteID,databaseID) }
	if err!=nil { _=tx.Rollback();apiError(w,shared.ErrInternal("database created but credential metadata could not be saved; do not retry blindly"));return }
	if err=tx.Commit();err!=nil { apiError(w,shared.ErrInternal("database created but metadata commit failed; inspect database inventory before retrying"));return }
	_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),DeviceID:shared.PtrInt64(deviceID),Action:"database_created",Details:map[string]interface{}{"database_id":databaseID,"server_id":in.ServerID,"database":in.Name,"username":in.Username}})
	writeJSON(w,http.StatusCreated,map[string]interface{}{"ok":true,"database":map[string]interface{}{"id":databaseID,"server_id":in.ServerID,"name":in.Name,"username":in.Username,"status":"ready"},"message":"Database and least-privilege application role created. Password stored encrypted."})
}

func (s *Server) siteDatabaseEnvironment(ctx context.Context, userID, deviceID int64, siteID string) (map[string]string,error) {
	rows,err:=s.db.conn.QueryContext(ctx,`SELECT d.name,ds.port,dc.username,dc.ciphertext,dc.nonce
		FROM site_database_connections c
		JOIN databases d ON d.id=c.database_id
		JOIN database_servers ds ON ds.id=d.server_id
		JOIN database_credentials dc ON dc.database_id=d.id
		WHERE c.user_id=? AND c.device_id=? AND c.site_id=? AND d.status='ready' AND ds.status='ready'
		ORDER BY c.id`,userID,deviceID,siteID)
	if err!=nil{return nil,err};defer rows.Close()
	type databaseEnv struct { Name string `json:"name"`; Host string `json:"host"`; Port int `json:"port"`; Username string `json:"username"`; Password string `json:"password"` }
	all:=make([]databaseEnv,0)
	for rows.Next(){
		var item databaseEnv;var ciphertext,nonce []byte
		if err:=rows.Scan(&item.Name,&item.Port,&item.Username,&ciphertext,&nonce);err!=nil{return nil,err}
		plain,err:=decryptCredential(s.credentialKey,nonce,ciphertext);if err!=nil{return nil,err}
		item.Host="127.0.0.1";item.Password=string(plain);all=append(all,item)
	}
	if err:=rows.Err();err!=nil{return nil,err}
	if len(all)==0{return nil,nil}
	encoded,err:=json.Marshal(all);if err!=nil{return nil,err}
	first:=all[0]
	return map[string]string{"DB_CONNECTION":"pgsql","DB_HOST":first.Host,"DB_PORT":strconv.Itoa(first.Port),"DB_DATABASE":first.Name,"DB_USERNAME":first.Username,"DB_PASSWORD":first.Password,"VELTRIX_DATABASES":string(encoded)},nil
}

func (s *Server) handleDatabaseConnections(w http.ResponseWriter,r *http.Request) {
	u:=UserFromContext(r.Context())
	if r.Method==http.MethodGet {
		deviceID,_:=strconv.ParseInt(r.URL.Query().Get("device_id"),10,64)
		siteID:=r.URL.Query().Get("site_id")
		if deviceID<1 || siteID=="" { apiError(w,shared.ErrBadRequest("device_id and site_id are required"));return }
		rows,err:=s.db.conn.QueryContext(r.Context(),`SELECT c.id,c.database_id,d.name,ds.port,dc.username FROM site_database_connections c JOIN databases d ON d.id=c.database_id JOIN database_servers ds ON ds.id=d.server_id JOIN database_credentials dc ON dc.database_id=d.id WHERE c.user_id=? AND c.device_id=? AND c.site_id=? ORDER BY c.id`,u.ID,deviceID,siteID)
		if err!=nil { apiError(w,shared.ErrInternal("could not load site database connections"));return };defer rows.Close()
		out:=make([]map[string]interface{},0)
		for rows.Next(){var id,databaseID int64;var name,username string;var port int;if err:=rows.Scan(&id,&databaseID,&name,&port,&username);err!=nil{apiError(w,shared.ErrInternal("could not read connections"));return};out=append(out,map[string]interface{}{"id":id,"database_id":databaseID,"database":name,"username":username,"host":"127.0.0.1","port":port})}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"connections":out});return
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
