package control

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"veltrix/internal/shared"
)

func (s *Server) handleProjectEnvironments(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/projects/"), "/"), "/")
	if len(parts) < 2 || parts[1] != "environments" {
		http.NotFound(w, r)
		return
	}
	projectID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || projectID < 1 {
		apiError(w, shared.ErrBadRequest("invalid project id"))
		return
	}
	action := "environment.read"
	if r.Method != http.MethodGet { action = "project.manage" }
	ownerID, accessErr := s.authorizeProjectOwner(r.Context(), projectID, u.ID, action)
	if accessErr != nil { apiError(w, accessErr); return }

	if len(parts) == 2 {
		switch r.Method {
		case http.MethodGet:
			rows, err := s.db.conn.QueryContext(r.Context(), "SELECT id,name,slug,description,is_protected,created_at,updated_at FROM project_environments WHERE project_id=? ORDER BY CASE slug WHEN 'production' THEN 0 WHEN 'development' THEN 1 WHEN 'staging' THEN 2 ELSE 3 END,name", projectID)
			if err != nil {
				apiError(w, shared.ErrInternal("could not load project environments"))
				return
			}
			defer rows.Close()
			items := make([]map[string]interface{}, 0)
			for rows.Next() {
				var id int64
				var name, slug, description string
				var protected bool
				var created, updated interface{}
				if err := rows.Scan(&id, &name, &slug, &description, &protected, &created, &updated); err != nil {
					apiError(w, shared.ErrInternal("could not read project environments"))
					return
				}
				items = append(items, map[string]interface{}{"id": id, "name": name, "slug": slug, "description": description, "is_protected": protected, "created_at": created, "updated_at": updated})
			}
			if err := rows.Err(); err != nil {
				apiError(w, shared.ErrInternal("could not read project environments"))
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "environments": items})
		case http.MethodPost:
			var in struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			}
			if err := decodeJSON(r, &in); err != nil {
				apiError(w, shared.ErrBadRequest("invalid JSON"))
				return
			}
			in.Name = strings.TrimSpace(in.Name)
			if len(in.Name) < 2 || len(in.Name) > 64 {
				apiError(w, shared.ErrBadRequest("environment name must be 2–64 characters"))
				return
			}
			slug := projectSlug(in.Name)
			if slug == "" {
				apiError(w, shared.ErrBadRequest("environment name must contain letters or numbers"))
				return
			}
			res, err := s.db.conn.ExecContext(r.Context(), "INSERT INTO project_environments(project_id,name,slug,description) VALUES(?,?,?,?)", projectID, in.Name, slug, strings.TrimSpace(in.Description))
			if err != nil {
				apiError(w, shared.ErrBadRequest("environment name is already in use"))
				return
			}
			id, _ := res.LastInsertId()
			_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),WorkspaceOwnerID:shared.PtrInt64(ownerID),Action:"project_environment_created",Details:map[string]interface{}{"project_id":projectID,"environment_id":id,"name":in.Name},IP:clientIP(r)})
			writeJSON(w, http.StatusCreated, map[string]interface{}{"ok": true, "environment": map[string]interface{}{"id": id, "name": in.Name, "slug": slug, "description": in.Description}})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}


	if len(parts) == 3 {
		environmentID, parseErr := strconv.ParseInt(parts[2], 10, 64)
		if parseErr != nil || environmentID < 1 { apiError(w, shared.ErrBadRequest("invalid environment id")); return }
		var name, slug, description string
		var protected bool
		err := s.db.conn.QueryRowContext(r.Context(), "SELECT name,slug,description,is_protected FROM project_environments WHERE id=? AND project_id=?", environmentID, projectID).Scan(&name, &slug, &description, &protected)
		if err == sql.ErrNoRows { apiError(w, shared.ErrNotFound("environment not found")); return }
		if err != nil { apiError(w, shared.ErrInternal("could not load environment")); return }
		switch r.Method {
		case http.MethodPatch:
			var in struct { Name string `json:"name"`; Description string `json:"description"` }
			if err := decodeJSON(r, &in); err != nil { apiError(w, shared.ErrBadRequest("invalid JSON")); return }
			in.Name = strings.TrimSpace(in.Name)
			if len(in.Name) < 2 || len(in.Name) > 64 { apiError(w, shared.ErrBadRequest("environment name must be 2–64 characters")); return }
			newSlug := projectSlug(in.Name)
			if newSlug == "" { apiError(w, shared.ErrBadRequest("environment name must contain letters or numbers")); return }
			if protected && newSlug != slug { apiError(w, shared.ErrBadRequest("Production is protected and cannot be renamed")); return }
			if protected { in.Name = name; newSlug = slug }
			_, err := s.db.conn.ExecContext(r.Context(), "UPDATE project_environments SET name=?,slug=?,description=? WHERE id=? AND project_id=?", in.Name, newSlug, strings.TrimSpace(in.Description), environmentID, projectID)
			if err != nil { apiError(w, shared.ErrBadRequest("environment name is already in use or could not be updated")); return }
			_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),WorkspaceOwnerID:shared.PtrInt64(ownerID),Action:"project_environment_updated",Details:map[string]interface{}{"project_id":projectID,"environment_id":environmentID,"name":in.Name},IP:clientIP(r)})
			writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
		case http.MethodDelete:
			if protected || slug == "production" { apiError(w, shared.ErrBadRequest("Production is protected and cannot be deleted")); return }
			tx, err := s.db.conn.BeginTx(r.Context(), nil)
			if err != nil { apiError(w, shared.ErrInternal("could not start environment deletion")); return }
			if _, err = tx.ExecContext(r.Context(), "DELETE FROM project_environment_sites WHERE project_id=? AND environment_id=?", projectID, environmentID); err == nil {
				_, err = tx.ExecContext(r.Context(), "DELETE FROM project_environments WHERE id=? AND project_id=? AND is_protected=FALSE", environmentID, projectID)
			}
			if err != nil { _ = tx.Rollback(); apiError(w, shared.ErrInternal("could not delete environment")); return }
			if err = tx.Commit(); err != nil { apiError(w, shared.ErrInternal("could not commit environment deletion")); return }
			_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),WorkspaceOwnerID:shared.PtrInt64(ownerID),Action:"project_environment_deleted",Details:map[string]interface{}{"project_id":projectID,"environment_id":environmentID},IP:clientIP(r)})
			writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	if len(parts) == 4 && parts[3] == "sites" {
		environmentID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil || environmentID < 1 {
			apiError(w, shared.ErrBadRequest("invalid environment id"))
			return
		}
		var valid int
		err = s.db.conn.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM project_environments WHERE id=? AND project_id=?", environmentID, projectID).Scan(&valid)
		if err != nil || valid != 1 {
			apiError(w, shared.ErrNotFound("environment not found"))
			return
		}
		switch r.Method {
		case http.MethodGet:
			rows, err := s.db.conn.QueryContext(r.Context(), "SELECT es.device_id,es.site_id,COALESCE(d.hostname,'Android host') FROM project_environment_sites es JOIN devices d ON d.id=es.device_id WHERE es.environment_id=? AND es.project_id=? AND d.user_id=? ORDER BY es.site_id", environmentID, projectID, ownerID)
			if err != nil {
				apiError(w, shared.ErrInternal("could not load environment applications"))
				return
			}
			defer rows.Close()
			items := make([]map[string]interface{}, 0)
			for rows.Next() {
				var deviceID int64
				var siteID, deviceName string
				if err := rows.Scan(&deviceID, &siteID, &deviceName); err != nil {
					apiError(w, shared.ErrInternal("could not read environment applications"))
					return
				}
				items = append(items, map[string]interface{}{"device_id": deviceID, "site_id": siteID, "device_name": deviceName})
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "sites": items})
		case http.MethodPost:
			var in struct {
				DeviceID int64  `json:"device_id"`
				SiteID   string `json:"site_id"`
			}
			if err := decodeJSON(r, &in); err != nil {
				apiError(w, shared.ErrBadRequest("invalid JSON"))
				return
			}
			if in.DeviceID < 1 || len(in.SiteID) < 1 || len(in.SiteID) > 64 {
				apiError(w, shared.ErrBadRequest("invalid application reference"))
				return
			}
			var linked int
			err = s.db.conn.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM project_sites ps JOIN devices d ON d.id=ps.device_id JOIN sites st ON st.device_id=ps.device_id AND st.site_id=ps.site_id WHERE ps.project_id=? AND ps.device_id=? AND ps.site_id=? AND d.user_id=?", projectID, in.DeviceID, in.SiteID, ownerID).Scan(&linked)
			if err != nil || linked != 1 {
				apiError(w, shared.ErrNotFound("application is not assigned to this project"))
				return
			}
			_, err = s.db.conn.ExecContext(r.Context(), "INSERT IGNORE INTO project_environment_sites(project_id,environment_id,device_id,site_id) VALUES(?,?,?,?)", projectID, environmentID, in.DeviceID, in.SiteID)
			if err != nil {
				apiError(w, shared.ErrInternal("could not assign application to environment"))
				return
			}
			_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),WorkspaceOwnerID:shared.PtrInt64(ownerID),Action:"project_environment_site_attached",Details:map[string]interface{}{"project_id":projectID,"environment_id":environmentID,"device_id":in.DeviceID,"site_id":in.SiteID},IP:clientIP(r)})
			writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
		case http.MethodDelete:
			// Production assignments are protected just like the environment itself.
			var protected bool
			if err := s.db.conn.QueryRowContext(r.Context(), "SELECT is_protected FROM project_environments WHERE id=? AND project_id=?", environmentID, projectID).Scan(&protected); err != nil {
				apiError(w, shared.ErrNotFound("environment not found"))
				return
			}
			if protected {
				apiError(w, shared.ErrBadRequest("Production application assignments are protected"))
				return
			}
			var in struct {
				DeviceID int64  `json:"device_id"`
				SiteID   string `json:"site_id"`
			}
			if err := decodeJSON(r, &in); err != nil || in.DeviceID < 1 || len(in.SiteID) < 1 || len(in.SiteID) > 64 {
				apiError(w, shared.ErrBadRequest("invalid application reference"))
				return
			}
			res, err := s.db.conn.ExecContext(r.Context(), "DELETE FROM project_environment_sites WHERE project_id=? AND environment_id=? AND device_id=? AND site_id=?", projectID, environmentID, in.DeviceID, in.SiteID)
			if err != nil {
				apiError(w, shared.ErrInternal("could not remove application from environment"))
				return
			}
			deleted, err := res.RowsAffected()
			if err != nil {
				apiError(w, shared.ErrInternal("could not verify environment assignment removal"))
				return
			}
			if deleted == 0 {
				apiError(w, shared.ErrNotFound("application assignment not found"))
				return
			}
			_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),WorkspaceOwnerID:shared.PtrInt64(ownerID),Action:"project_environment_site_detached",Details:map[string]interface{}{"project_id":projectID,"environment_id":environmentID,"device_id":in.DeviceID,"site_id":in.SiteID},IP:clientIP(r)})
			writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	if len(parts) == 4 && parts[3] == "databases" {
		environmentID, parseErr := strconv.ParseInt(parts[2], 10, 64)
		if parseErr != nil || environmentID < 1 { apiError(w, shared.ErrBadRequest("invalid environment id")); return }
		var protected bool
		if err := s.db.conn.QueryRowContext(r.Context(), "SELECT is_protected FROM project_environments WHERE id=? AND project_id=?", environmentID, projectID).Scan(&protected); err != nil {
			apiError(w, shared.ErrNotFound("environment not found")); return
		}
		switch r.Method {
		case http.MethodGet:
			rows, err := s.db.conn.QueryContext(r.Context(), `SELECT ds.id,ds.device_id,ds.name,ds.engine,ds.status
				FROM project_environment_database_servers eds
				JOIN database_servers ds ON ds.id=eds.database_server_id
				WHERE eds.project_id=? AND eds.environment_id=? AND ds.user_id=?
				ORDER BY ds.name`, projectID, environmentID, ownerID)
			if err != nil { apiError(w, shared.ErrInternal("could not load environment database servers")); return }
			defer rows.Close()
			items := make([]map[string]interface{}, 0)
			for rows.Next() {
				var serverID, deviceID int64
				var name, engine, status string
				if err := rows.Scan(&serverID,&deviceID,&name,&engine,&status); err != nil { apiError(w, shared.ErrInternal("could not read environment database servers")); return }
				items = append(items, map[string]interface{}{"id":serverID,"device_id":deviceID,"name":name,"engine":engine,"status":status})
			}
			if err := rows.Err(); err != nil { apiError(w, shared.ErrInternal("could not read environment database servers")); return }
			writeJSON(w, http.StatusOK, map[string]interface{}{"ok":true,"database_servers":items})
		case http.MethodPost:
			var in struct { ServerID int64 `json:"server_id"` }
			if err := decodeJSON(r, &in); err != nil || in.ServerID < 1 { apiError(w, shared.ErrBadRequest("invalid database server reference")); return }
			var linked int
			if err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM project_database_servers pds JOIN database_servers ds ON ds.id=pds.database_server_id WHERE pds.project_id=? AND pds.database_server_id=? AND ds.user_id=?`, projectID, in.ServerID, ownerID).Scan(&linked); err != nil || linked != 1 {
				apiError(w, shared.ErrNotFound("database server is not assigned to this project")); return
			}
			if _, err := s.db.conn.ExecContext(r.Context(), `INSERT IGNORE INTO project_environment_database_servers(project_id,environment_id,database_server_id) VALUES(?,?,?)`, projectID, environmentID, in.ServerID); err != nil {
				apiError(w, shared.ErrInternal("could not assign database server to environment")); return
			}
			_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),WorkspaceOwnerID:shared.PtrInt64(ownerID),Action:"project_environment_database_attached",Details:map[string]interface{}{"project_id":projectID,"environment_id":environmentID,"database_server_id":in.ServerID},IP:clientIP(r)})
			writeJSON(w, http.StatusOK, map[string]interface{}{"ok":true})
		case http.MethodDelete:
			if protected { apiError(w, shared.ErrBadRequest("Production database assignments are protected")); return }
			var in struct { ServerID int64 `json:"server_id"` }
			if err := decodeJSON(r, &in); err != nil || in.ServerID < 1 { apiError(w, shared.ErrBadRequest("invalid database server reference")); return }
			res, err := s.db.conn.ExecContext(r.Context(), `DELETE FROM project_environment_database_servers WHERE project_id=? AND environment_id=? AND database_server_id=?`, projectID, environmentID, in.ServerID)
			if err != nil { apiError(w, shared.ErrInternal("could not remove database server from environment")); return }
			deleted, err := res.RowsAffected()
			if err != nil { apiError(w, shared.ErrInternal("could not verify database assignment removal")); return }
			if deleted == 0 { apiError(w, shared.ErrNotFound("database server assignment not found")); return }
			_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),WorkspaceOwnerID:shared.PtrInt64(ownerID),Action:"project_environment_database_detached",Details:map[string]interface{}{"project_id":projectID,"environment_id":environmentID,"database_server_id":in.ServerID},IP:clientIP(r)})
			writeJSON(w, http.StatusOK, map[string]interface{}{"ok":true})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	if len(parts) == 4 && parts[3] == "database-instances" {
		environmentID, parseErr := strconv.ParseInt(parts[2], 10, 64)
		if parseErr != nil || environmentID < 1 { apiError(w, shared.ErrBadRequest("invalid environment id")); return }
		var protected bool
		if err := s.db.conn.QueryRowContext(r.Context(), "SELECT is_protected FROM project_environments WHERE id=? AND project_id=?", environmentID, projectID).Scan(&protected); err != nil { apiError(w, shared.ErrNotFound("environment not found")); return }
		switch r.Method {
		case http.MethodGet:
			rows, err := s.db.conn.QueryContext(r.Context(), `SELECT d.id,d.name,d.status,d.created_at,ds.id,ds.name,ds.engine,ds.device_id
				FROM project_environment_databases edb JOIN db_instances d ON d.id=edb.database_id AND d.user_id=?
				JOIN database_servers ds ON ds.id=d.server_id AND ds.user_id=d.user_id
				WHERE edb.project_id=? AND edb.environment_id=? ORDER BY d.created_at DESC`, ownerID, projectID, environmentID)
			if err != nil { apiError(w, shared.ErrInternal("could not load environment databases")); return }; defer rows.Close()
			items := make([]map[string]interface{}, 0)
			for rows.Next() { var id,serverID,deviceID int64; var name,status,serverName,engine string; var created interface{}
				if err:=rows.Scan(&id,&name,&status,&created,&serverID,&serverName,&engine,&deviceID);err!=nil{apiError(w,shared.ErrInternal("could not read environment databases"));return}
				items=append(items,map[string]interface{}{"id":id,"name":name,"status":status,"created_at":created,"server_id":serverID,"server_name":serverName,"engine":engine,"device_id":deviceID}) }
			if err:=rows.Err();err!=nil{apiError(w,shared.ErrInternal("could not read environment databases"));return}
			writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"databases":items})
		case http.MethodPost:
			var in struct{DatabaseID int64 `json:"database_id"`}
			if err:=decodeJSON(r,&in);err!=nil||in.DatabaseID<1{apiError(w,shared.ErrBadRequest("invalid database reference"));return}
			var valid int
			err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM db_instances d
				JOIN database_servers ds ON ds.id=d.server_id AND ds.user_id=d.user_id
				JOIN project_database_servers pds ON pds.database_server_id=ds.id AND pds.project_id=?
				JOIN project_environment_database_servers eds ON eds.project_id=pds.project_id AND eds.database_server_id=ds.id AND eds.environment_id=?
				WHERE d.id=? AND d.user_id=?`,projectID,environmentID,in.DatabaseID,ownerID).Scan(&valid)
			if err!=nil||valid!=1{apiError(w,shared.ErrNotFound("database is not available through a server assigned to this project environment"));return}
			res,err:=s.db.conn.ExecContext(r.Context(),`INSERT IGNORE INTO project_environment_databases(project_id,environment_id,database_id) VALUES(?,?,?)`,projectID,environmentID,in.DatabaseID)
			if err!=nil{apiError(w,shared.ErrInternal("could not assign database to environment"));return}
			affected,err:=res.RowsAffected();if err!=nil{apiError(w,shared.ErrInternal("could not verify database assignment"));return}
			if affected==0{var current int64;lookupErr:=s.db.conn.QueryRowContext(r.Context(),`SELECT environment_id FROM project_environment_databases WHERE project_id=? AND database_id=?`,projectID,in.DatabaseID).Scan(&current);if lookupErr!=nil||current!=environmentID{apiError(w,shared.ErrBadRequest("database is already assigned to another environment"));return}}
			_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),WorkspaceOwnerID:shared.PtrInt64(ownerID),Action:"project_environment_database_instance_attached",Details:map[string]interface{}{"project_id":projectID,"environment_id":environmentID,"database_id":in.DatabaseID},IP:clientIP(r)})
			writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true})
		case http.MethodDelete:
			if protected{apiError(w,shared.ErrBadRequest("Production database assignments are protected"));return}
			var in struct{DatabaseID int64 `json:"database_id"`}
			if err:=decodeJSON(r,&in);err!=nil||in.DatabaseID<1{apiError(w,shared.ErrBadRequest("invalid database reference"));return}
			res,err:=s.db.conn.ExecContext(r.Context(),`DELETE FROM project_environment_databases WHERE project_id=? AND environment_id=? AND database_id=?`,projectID,environmentID,in.DatabaseID)
			if err!=nil{apiError(w,shared.ErrInternal("could not remove database from environment"));return}
			deleted,err:=res.RowsAffected();if err!=nil{apiError(w,shared.ErrInternal("could not verify database assignment removal"));return};if deleted==0{apiError(w,shared.ErrNotFound("database assignment not found"));return}
			_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),WorkspaceOwnerID:shared.PtrInt64(ownerID),Action:"project_environment_database_instance_detached",Details:map[string]interface{}{"project_id":projectID,"environment_id":environmentID,"database_id":in.DatabaseID},IP:clientIP(r)})
			writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true})
		default:http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
		}
		return
	}
	http.NotFound(w, r)
}
