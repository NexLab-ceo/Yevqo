package control

import (
	"database/sql"
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"veltrix/internal/shared"
)

func (s *Server) handleWorkspaceMembers(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil { apiError(w, shared.ErrUnauthorized("not logged in")); return }
	switch r.Method {
	case http.MethodGet:
		rows, err := s.db.conn.QueryContext(r.Context(), `
			SELECT wm.id,wm.member_user_id,u.username,u.email,wm.role,wm.status,wm.created_at
			FROM workspace_members wm JOIN users u ON u.id=wm.member_user_id
			WHERE wm.owner_user_id=? ORDER BY wm.created_at DESC`, user.ID)
		if err != nil { apiError(w, shared.ErrInternal("could not load workspace members")); return }
		defer rows.Close()
		type member struct {
			ID int64 `json:"id"`
			UserID int64 `json:"user_id"`
			Username string `json:"username"`
			Email string `json:"email"`
			Role string `json:"role"`
			Status string `json:"status"`
			CreatedAt sql.NullTime `json:"-"`
		}
		out := make([]map[string]interface{},0)
		for rows.Next() {
			var m member
			if err := rows.Scan(&m.ID,&m.UserID,&m.Username,&m.Email,&m.Role,&m.Status,&m.CreatedAt); err != nil { apiError(w, shared.ErrInternal("could not read workspace members")); return }
			out=append(out,map[string]interface{}{"id":m.ID,"user_id":m.UserID,"username":m.Username,"email":m.Email,"role":m.Role,"status":m.Status,"created_at":m.CreatedAt.Time})
		}
		if err:=rows.Err();err!=nil{apiError(w,shared.ErrInternal("could not read workspace members"));return}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"members":out})
	case http.MethodPost:
		var in struct { Email string `json:"email"`; Role string `json:"role"` }
		if err:=decodeJSON(r,&in);err!=nil{apiError(w,shared.ErrBadRequest("invalid JSON"));return}
		in.Email=strings.TrimSpace(strings.ToLower(in.Email))
		address,err:=mail.ParseAddress(in.Email)
		if err!=nil||address.Address!=in.Email{apiError(w,shared.ErrBadRequest("enter a valid email address"));return}
		in.Role=strings.ToLower(strings.TrimSpace(in.Role))
		if in.Role!="admin"&&in.Role!="developer"{apiError(w,shared.ErrBadRequest("role must be Admin or Developer"));return}
		var memberID int64
		err=s.db.conn.QueryRowContext(r.Context(),`SELECT id FROM users WHERE email=?`,in.Email).Scan(&memberID)
		if err==sql.ErrNoRows{apiError(w,shared.ErrNotFound("user must create an account before being invited"));return}
		if err!=nil{apiError(w,shared.ErrInternal("could not find invited user"));return}
		if memberID==user.ID{apiError(w,shared.ErrBadRequest("workspace owner cannot invite themselves"));return}
		_,err=s.db.conn.ExecContext(r.Context(),`INSERT INTO workspace_members(owner_user_id,member_user_id,role,status,invited_email) VALUES(?,?,?,'invited',?) ON DUPLICATE KEY UPDATE role=VALUES(role),status='invited',invited_email=VALUES(invited_email)`,user.ID,memberID,in.Role,in.Email)
		if err!=nil{apiError(w,shared.ErrInternal("could not create workspace invitation"));return}
		_,_=s.db.conn.ExecContext(r.Context(),`INSERT INTO notifications(user_id,type,title,body) VALUES(?,?,?,?)`,memberID,"info","Workspace invitation","You have been invited to a NexLab workspace as "+strings.Title(in.Role)+". Accept it in Team & Permissions.")
		_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(user.ID),Action:"workspace_member_invited",Details:map[string]interface{}{"member_user_id":memberID,"role":in.Role},IP:clientIP(r)})
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"message":"invitation sent"})
	default:
		http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleWorkspaceMemberByID(w http.ResponseWriter,r *http.Request) {
	user:=UserFromContext(r.Context())
	if user==nil{apiError(w,shared.ErrUnauthorized("not logged in"));return}
	if r.Method!=http.MethodDelete{http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return}
	raw:=strings.TrimPrefix(r.URL.Path,"/api/workspace/members/")
	id,err:=strconv.ParseInt(raw,10,64)
	if err!=nil||id<1{apiError(w,shared.ErrBadRequest("invalid member id"));return}
	result,err:=s.db.conn.ExecContext(r.Context(),`DELETE FROM workspace_members WHERE id=? AND owner_user_id=?`,id,user.ID)
	if err!=nil{apiError(w,shared.ErrInternal("could not remove workspace member"));return}
	affected,_:=result.RowsAffected()
	if affected==0{apiError(w,shared.ErrNotFound("workspace member not found"));return}
	_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(user.ID),Action:"workspace_member_removed",Details:map[string]interface{}{"membership_id":id},IP:clientIP(r)})
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true})
}

func (s *Server) handleWorkspaceInvitations(w http.ResponseWriter,r *http.Request) {
	user:=UserFromContext(r.Context())
	if user==nil{apiError(w,shared.ErrUnauthorized("not logged in"));return}
	switch r.Method{
	case http.MethodGet:
		rows,err:=s.db.conn.QueryContext(r.Context(),`SELECT wm.id,wm.owner_user_id,u.username,u.email,wm.role,wm.created_at FROM workspace_members wm JOIN users u ON u.id=wm.owner_user_id WHERE wm.member_user_id=? AND wm.status='invited' ORDER BY wm.created_at DESC`,user.ID)
		if err!=nil{apiError(w,shared.ErrInternal("could not load invitations"));return}
		defer rows.Close()
		out:=make([]map[string]interface{},0)
		for rows.Next(){var id,ownerID int64;var username,email,role string;var created sql.NullTime;if err:=rows.Scan(&id,&ownerID,&username,&email,&role,&created);err!=nil{apiError(w,shared.ErrInternal("could not read invitations"));return};out=append(out,map[string]interface{}{"id":id,"owner_user_id":ownerID,"owner_username":username,"owner_email":email,"role":role,"created_at":created.Time})}
		if err:=rows.Err();err!=nil{apiError(w,shared.ErrInternal("could not read invitations"));return}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"invitations":out})
	case http.MethodPost:
		var in struct{ID int64 `json:"id"`}
		if err:=decodeJSON(r,&in);err!=nil||in.ID<1{apiError(w,shared.ErrBadRequest("invalid invitation id"));return}
		result,err:=s.db.conn.ExecContext(r.Context(),`UPDATE workspace_members SET status='active' WHERE id=? AND member_user_id=? AND status='invited'`,in.ID,user.ID)
		if err!=nil{apiError(w,shared.ErrInternal("could not accept invitation"));return}
		affected,_:=result.RowsAffected();if affected==0{apiError(w,shared.ErrNotFound("invitation not found"));return}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"message":"workspace joined"})
	default:
		http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
	}
}
