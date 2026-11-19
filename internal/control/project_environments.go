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
	if r.Method != http.MethodGet {
		action = "project.manage"
	}
	if err := s.authorizeProjectAction(r.Context(), projectID, u.ID, action); err != nil {
		if err == sql.ErrNoRows {
			apiError(w, shared.ErrNotFound("project not found"))
		} else if strings.Contains(err.Error(), "permission denied") {
			apiError(w, shared.ErrForbidden("workspace permission denied"))
		} else {
			apiError(w, shared.ErrInternal("could not verify project permissions"))
		}
		return
	}

	var ownerID int64
	if err := s.db.conn.QueryRowContext(r.Context(), "SELECT user_id FROM projects WHERE id=?", projectID).Scan(&ownerID); err != nil {
		apiError(w, shared.ErrNotFound("project not found"))
		return
	}

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
			writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	http.NotFound(w, r)
}
