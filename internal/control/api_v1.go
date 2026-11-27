package control

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"veltrix/internal/shared"
)

func apiKeyDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *Server) canUseProFeatures(user *shared.User) bool {
	return user != nil && (user.Username == s.adminUsername || user.Status == shared.UserPro)
}

func (s *Server) handleAPIKeys(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if !s.canUseProFeatures(user) {
		apiError(w, shared.ErrForbidden("Yevqo API is available on the Pro plan"))
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := s.db.conn.QueryContext(r.Context(), `SELECT id,name,key_prefix,created_at,last_used_at,revoked_at FROM api_keys WHERE user_id=? AND revoked_at IS NULL ORDER BY created_at DESC`, user.ID)
		if err != nil { apiError(w, shared.ErrInternal("could not load API keys")); return }
		defer rows.Close()
		items := make([]map[string]interface{}, 0)
		for rows.Next() {
			var id int64
			var name, prefix string
			var created time.Time
			var lastUsed, revoked *time.Time
			if err := rows.Scan(&id,&name,&prefix,&created,&lastUsed,&revoked); err != nil { apiError(w,shared.ErrInternal("could not read API keys")); return }
			items=append(items,map[string]interface{}{"id":id,"name":name,"prefix":prefix,"created_at":created,"last_used_at":lastUsed,"revoked_at":revoked})
		}
		if err:=rows.Err();err!=nil { apiError(w,shared.ErrInternal("could not read API keys")); return }
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"keys":items})
	case http.MethodPost:
		var in struct { Name string `json:"name"` }
		if err:=decodeJSON(r,&in);err!=nil { apiError(w,shared.ErrBadRequest("invalid JSON")); return }
		in.Name=strings.TrimSpace(in.Name)
		if len(in.Name)<1 || len(in.Name)>64 { apiError(w,shared.ErrBadRequest("API key name must be 1-64 characters")); return }
		random,err:=GenerateToken(32)
		if err!=nil { apiError(w,shared.ErrInternal("could not generate API key")); return }
		token:="yevqo_live_"+random
		prefix:=token[:19]
		_,err=s.db.conn.ExecContext(r.Context(),`INSERT INTO api_keys(user_id,name,key_prefix,key_hash) VALUES(?,?,?,?)`,user.ID,in.Name,prefix,apiKeyDigest(token))
		if err!=nil { apiError(w,shared.ErrInternal("could not create API key")); return }
		_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(user.ID),Action:"api_key_created",Details:map[string]interface{}{"name":in.Name,"prefix":prefix}})
		writeJSON(w,http.StatusCreated,map[string]interface{}{"ok":true,"key":token,"warning":"Copy this key now. It will not be shown again."})
	default:
		http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAPIKeyByID(w http.ResponseWriter,r *http.Request) {
	user:=UserFromContext(r.Context())
	if !s.canUseProFeatures(user) { apiError(w,shared.ErrForbidden("Yevqo API is available on the Pro plan")); return }
	if r.Method!=http.MethodDelete { http.Error(w,"method not allowed",http.StatusMethodNotAllowed); return }
	raw:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/api-keys/"),"/")
	id,err:=strconv.ParseInt(raw,10,64)
	if err!=nil || id<1 { apiError(w,shared.ErrBadRequest("invalid API key id")); return }
	res,err:=s.db.conn.ExecContext(r.Context(),`UPDATE api_keys SET revoked_at=UTC_TIMESTAMP() WHERE id=? AND user_id=? AND revoked_at IS NULL`,id,user.ID)
	if err!=nil { apiError(w,shared.ErrInternal("could not revoke API key")); return }
	n,_:=res.RowsAffected()
	if n==0 { apiError(w,shared.ErrNotFound("API key not found")); return }
	_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(user.ID),Action:"api_key_revoked",Details:map[string]interface{}{"api_key_id":id}})
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true})
}

func (s *Server) requireProAPI(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter,r *http.Request) {
		header:=strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(header,"Bearer ") { apiError(w,shared.ErrUnauthorized("missing Bearer API key")); return }
		token:=strings.TrimSpace(strings.TrimPrefix(header,"Bearer "))
		if len(token)<40 || len(token)>128 { apiError(w,shared.ErrUnauthorized("invalid API key")); return }
		var userID,keyID int64
		err:=s.db.conn.QueryRowContext(r.Context(),`SELECT k.id,k.user_id FROM api_keys k JOIN users u ON u.id=k.user_id WHERE k.key_hash=? AND k.revoked_at IS NULL AND u.status<>'blocked' AND (u.status='pro' OR u.username=?)`,apiKeyDigest(token),s.adminUsername).Scan(&keyID,&userID)
		if err!=nil { apiError(w,shared.ErrUnauthorized("invalid or inactive API key")); return }
		user,err:=s.db.GetUserByID(r.Context(),userID)
		if err!=nil { apiError(w,shared.ErrUnauthorized("API account unavailable")); return }
		_,_=s.db.conn.ExecContext(r.Context(),`UPDATE api_keys SET last_used_at=UTC_TIMESTAMP() WHERE id=?`,keyID)
		ctx:=r.Context()
		ctx=context.WithValue(ctx,ctxKeyUser,user)
		next(w,r.WithContext(ctx))
	}
}

func (s *Server) handleV1(w http.ResponseWriter,r *http.Request) {
	path:=strings.TrimPrefix(r.URL.Path,"/v1")
	if path=="/me" {
		u:=UserFromContext(r.Context())
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"user":map[string]interface{}{"id":u.ID,"username":u.Username,"plan":u.Status,"is_admin":u.Username==s.adminUsername}})
		return
	}
	r2:=r.Clone(r.Context())
	switch {
	case path=="/projects" || strings.HasPrefix(path,"/projects/"):
		r2.URL.Path="/api"+path
		if strings.Contains(path,"/repository") { s.handleProjectRepository(w,r2); return }
		if strings.Contains(path,"/deployments") { s.handleProjectDeployments(w,r2); return }
		if strings.Contains(path,"/environments") { s.handleProjectEnvironments(w,r2); return }
		if strings.Contains(path,"/members") { s.handleProjectMembers(w,r2); return }
		s.handleProjectByIDOrCollection(w,r2)
	case path=="/devices" || strings.HasPrefix(path,"/devices/"):
		r2.URL.Path="/api"+path
		if strings.HasPrefix(path,"/devices/") { s.handleDeviceByID(w,r2) } else { s.handleDevices(w,r2) }
	case path=="/sites" || strings.HasPrefix(path,"/sites/"):
		r2.URL.Path="/api"+path
		if strings.HasPrefix(path,"/sites/") { s.handleSiteByID(w,r2) } else { s.handleSites(w,r2) }
	case path=="/deployments": s.handleDeployments(w,r2)
	case path=="/databases": s.handleDatabaseCollection(w,r2)
	case path=="/databases/servers": s.handleDatabaseServers(w,r2)
	case strings.HasPrefix(path,"/databases/servers/"): r2.URL.Path="/api"+path; s.handleDatabaseServerByID(w,r2)
	case strings.HasPrefix(path,"/databases/"): r2.URL.Path="/api"+path; s.handleDatabaseByID(w,r2)
	case path=="/backups": s.handleBackups(w,r2)
	case path=="/domains": s.handleDomains(w,r2)
	case path=="/monitoring": s.handleMonitoring(w,r2)
	case path=="/logs": s.handleLogs(w,r2)
	case path=="/reports": s.handleReports(w,r2)
	default: http.NotFound(w,r)
	}
}

func (s *Server) handleProjectByIDOrCollection(w http.ResponseWriter,r *http.Request) {
	if r.URL.Path=="/api/projects" { s.handleProjects(w,r); return }
	s.handleProjectByID(w,r)
}
