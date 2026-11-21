package control

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"veltrix/internal/shared"
)

func (s *Server) handleProjectMembers(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil { apiError(w, shared.ErrUnauthorized("not logged in")); return }
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/projects/"), "/"), "/")
	if len(parts) < 3 || parts[1] != "members" { http.NotFound(w, r); return }
	projectID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || projectID < 1 { apiError(w, shared.ErrBadRequest("invalid project id")); return }

	if len(parts) == 3 && r.Method == http.MethodGet {
		ownerID, err := s.authorizeProjectOwner(r.Context(), projectID, user.ID, "project.read")
		if err != nil { apiError(w, err); return }
		var ownerName, ownerEmail string
		if err := s.db.conn.QueryRowContext(r.Context(), "SELECT username,email FROM users WHERE id=?", ownerID).Scan(&ownerName,&ownerEmail); err != nil {
			apiError(w, shared.ErrInternal("could not load project owner")); return
		}
		items := []map[string]interface{}{{"user_id":ownerID,"username":ownerName,"email":ownerEmail,"workspace_role":"owner","project_role":"owner","scope":"project","status":"active"}}
		rows, err := s.db.conn.QueryContext(r.Context(), `SELECT wm.member_user_id,u.username,u.email,wm.role,wm.status,pm.role
			FROM workspace_members wm JOIN users u ON u.id=wm.member_user_id
			LEFT JOIN project_members pm ON pm.project_id=? AND pm.member_user_id=wm.member_user_id
			WHERE wm.owner_user_id=? AND wm.status='active' ORDER BY u.username`, projectID, ownerID)
		if err != nil { apiError(w, shared.ErrInternal("could not load project members")); return }
		defer rows.Close()
		for rows.Next() {
			var id int64; var username,email,workspaceRole,status string; var projectRole sql.NullString
			if err := rows.Scan(&id,&username,&email,&workspaceRole,&status,&projectRole); err != nil { apiError(w, shared.ErrInternal("could not read project members")); return }
			role, scope := workspaceRole, "inherited"
			if projectRole.Valid { role, scope = projectRole.String, "project" }
			items = append(items,map[string]interface{}{"user_id":id,"username":username,"email":email,"workspace_role":workspaceRole,"project_role":role,"scope":scope,"status":status})
		}
		if err := rows.Err(); err != nil { apiError(w, shared.ErrInternal("could not read project members")); return }
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"members":items})
		return
	}

	ownerID, err := s.authorizeProjectOwner(r.Context(), projectID, user.ID, "project.manage")
	if err != nil { apiError(w, err); return }
	if len(parts) == 3 && r.Method == http.MethodPost {
		var in struct { UserID int64 `json:"user_id"`; Role string `json:"role"` }
		if err := decodeJSON(r,&in); err != nil || in.UserID < 1 { apiError(w, shared.ErrBadRequest("invalid project member")); return }
		in.Role = strings.ToLower(strings.TrimSpace(in.Role))
		if in.Role != "admin" && in.Role != "developer" && in.Role != "viewer" { apiError(w, shared.ErrBadRequest("role must be Admin, Developer or Viewer")); return }
		if in.UserID == ownerID { apiError(w, shared.ErrBadRequest("project owner cannot be assigned a member role")); return }
		_, allowed, roleErr := s.db.WorkspaceRole(r.Context(), ownerID, in.UserID)
		if roleErr != nil { apiError(w, shared.ErrInternal("could not verify workspace membership")); return }
		if !allowed { apiError(w, shared.ErrBadRequest("project members must already belong to the workspace")); return }
		_, err = s.db.conn.ExecContext(r.Context(), `INSERT INTO project_members(project_id,member_user_id,role,created_by) VALUES(?,?,?,?)
			ON DUPLICATE KEY UPDATE role=VALUES(role),updated_at=CURRENT_TIMESTAMP`, projectID,in.UserID,in.Role,user.ID)
		if err != nil { apiError(w, shared.ErrInternal("could not save project member role")); return }
		_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(user.ID),WorkspaceOwnerID:shared.PtrInt64(ownerID),Action:"project_member_role_updated",Details:map[string]interface{}{"project_id":projectID,"member_user_id":in.UserID,"role":in.Role},IP:clientIP(r)})
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true})
		return
	}
	if len(parts) == 4 && r.Method == http.MethodDelete {
		memberID, parseErr := strconv.ParseInt(parts[3],10,64)
		if parseErr != nil || memberID < 1 || memberID == ownerID { apiError(w, shared.ErrBadRequest("invalid project member id")); return }
		res, err := s.db.conn.ExecContext(r.Context(),"DELETE FROM project_members WHERE project_id=? AND member_user_id=?",projectID,memberID)
		if err != nil { apiError(w, shared.ErrInternal("could not remove project member role")); return }
		deleted, err := res.RowsAffected()
		if err != nil { apiError(w, shared.ErrInternal("could not verify project member removal")); return }
		if deleted == 0 { apiError(w, shared.ErrNotFound("project member assignment not found")); return }
		_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(user.ID),WorkspaceOwnerID:shared.PtrInt64(ownerID),Action:"project_member_role_removed",Details:map[string]interface{}{"project_id":projectID,"member_user_id":memberID},IP:clientIP(r)})
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true})
		return
	}
	http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
}
