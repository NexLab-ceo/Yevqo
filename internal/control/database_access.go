package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"veltrix/internal/shared"
)

// authorizeDatabaseServer resolves the owning workspace before any database
// server operation. Members never gain access merely by guessing a server ID.
func (s *Server) authorizeDatabaseServer(ctx context.Context, serverID, actorID int64, action string) (ownerID, deviceID int64, err error) {
	if serverID < 1 {
		return 0, 0, shared.ErrBadRequest("invalid database server id")
	}
	err = s.db.conn.QueryRowContext(ctx, "SELECT user_id,device_id FROM database_servers WHERE id=?", serverID).Scan(&ownerID, &deviceID)
	if err == sql.ErrNoRows {
		return 0, 0, shared.ErrNotFound("database server not found")
	}
	if err != nil {
		return 0, 0, shared.ErrInternal("could not resolve database server owner")
	}
	if err = s.authorizeDatabaseServerProjects(ctx, serverID, ownerID, actorID, action); err != nil {
		return 0, 0, err
	}
	return ownerID, deviceID, nil
}

 

// authorizeDatabaseServerProjects applies project-specific role overrides to a
// database server attached to one or more projects. A server not attached to a
// project keeps the workspace-level permission model. When shared by projects,
// the actor must be authorized for every attached project (fail closed).
func (s *Server) authorizeDatabaseServerProjects(ctx context.Context, serverID, ownerID, actorID int64, action string) error {
	rows, err := s.db.conn.QueryContext(ctx, `SELECT p.id,p.user_id
		FROM project_database_servers pds
		JOIN projects p ON p.id=pds.project_id
		WHERE pds.database_server_id=?
		ORDER BY p.id`, serverID)
	if err != nil {
		return shared.ErrInternal("could not resolve database server project access")
	}
	type projectOwner struct { id, ownerID int64 }
	projects := make([]projectOwner, 0)
	for rows.Next() {
		var item projectOwner
		if err := rows.Scan(&item.id, &item.ownerID); err != nil {
			rows.Close()
			return shared.ErrInternal("could not read database server project access")
		}
		projects = append(projects, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return shared.ErrInternal("could not read database server project access")
	}
	if err := rows.Close(); err != nil {
		return shared.ErrInternal("could not close database server project access")
	}
	if len(projects) == 0 {
		return s.authorizeWorkspace(ctx, ownerID, actorID, action)
	}
	for _, project := range projects {
		if project.ownerID != ownerID {
			return shared.ErrInternal("database server project ownership is inconsistent")
		}
		if err := s.authorizeProjectAction(ctx, project.id, actorID, action); err != nil {
			return err
		}
	}
	return nil
}

// authorizeDatabaseInstance resolves the database and its server together,
// rejecting inconsistent ownership before granting workspace access.
func (s *Server) authorizeDatabaseInstance(ctx context.Context, databaseID, actorID int64, action string) (ownerID, serverID, deviceID int64, err error) {
	if databaseID < 1 {
		return 0, 0, 0, shared.ErrBadRequest("invalid database id")
	}
	err = s.db.conn.QueryRowContext(ctx, `SELECT d.user_id,d.server_id,ds.device_id
		FROM db_instances d JOIN database_servers ds ON ds.id=d.server_id AND ds.user_id=d.user_id
		WHERE d.id=?`, databaseID).Scan(&ownerID, &serverID, &deviceID)
	if err == sql.ErrNoRows {
		return 0, 0, 0, shared.ErrNotFound("database not found")
	}
	if err != nil {
		return 0, 0, 0, shared.ErrInternal("could not resolve database owner")
	}
	if err = s.authorizeDatabaseServerProjects(ctx, serverID, ownerID, actorID, action); err != nil {
		return 0, 0, 0, err
	}
	return ownerID, serverID, deviceID, nil
}

// authorizeDeviceOwner resolves the workspace owner for a device-scoped
// resource and applies the requested role permission.
// authorizeSiteProjects applies project role overrides to a website attached
// to one or more projects. Unassigned sites retain workspace-level permissions.
func (s *Server) authorizeSiteProjects(ctx context.Context, deviceID int64, siteID string, ownerID, actorID int64, action string) error {
	rows, err := s.db.conn.QueryContext(ctx, `SELECT p.id,p.user_id FROM project_sites ps JOIN projects p ON p.id=ps.project_id WHERE ps.device_id=? AND ps.site_id=? ORDER BY p.id`, deviceID, siteID)
	if err != nil { return shared.ErrInternal("could not resolve website project access") }
	type projectOwner struct { id, ownerID int64 }
	projects := make([]projectOwner, 0)
	for rows.Next() { var item projectOwner; if err := rows.Scan(&item.id,&item.ownerID); err != nil { rows.Close(); return shared.ErrInternal("could not read website project access") }; projects=append(projects,item) }
	if err := rows.Err(); err != nil { rows.Close(); return shared.ErrInternal("could not read website project access") }
	if err := rows.Close(); err != nil { return shared.ErrInternal("could not close website project access") }
	if len(projects)==0 { return s.authorizeWorkspace(ctx,ownerID,actorID,action) }
	for _,project := range projects { if project.ownerID!=ownerID { return shared.ErrInternal("website project ownership is inconsistent") }; if err := s.authorizeProjectAction(ctx,project.id,actorID,action); err != nil { return err } }
	return nil
}

func projectLogDetailID(details map[string]interface{}, key string) (int64, bool) {
	raw, exists := details[key]
	if !exists { return 0, false }
	switch value := raw.(type) {
	case float64:
		id := int64(value)
		if value <= 0 || float64(id) != value { return 0, true }
		return id, true
	case string:
		id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil || id < 1 { return 0, true }
		return id, true
	default:
		return 0, true
	}
}

func isProjectAccessFilterError(err error) bool {
	var apiErr *shared.APIError
	return errors.As(err, &apiErr) && (apiErr.Code == http.StatusForbidden || apiErr.Code == http.StatusNotFound)
}

// authorizeProjectLogDetails prevents project-scoped logs from exposing activity
// for sites, databases or backups that the current project member cannot access.
func (s *Server) authorizeProjectLogDetails(ctx context.Context, projectID, ownerID, actorID, deviceID int64, raw []byte, siteAction string) error {
	if len(raw) == 0 { return nil }
	var details map[string]interface{}
	if err := json.Unmarshal(raw, &details); err != nil { return shared.ErrForbidden("log details cannot be scoped to this project") }

	if deviceID < 1 {
		if detailDeviceID, exists := projectLogDetailID(details, "device_id"); exists {
			if detailDeviceID < 1 { return shared.ErrForbidden("log has no valid device scope") }
			deviceID = detailDeviceID
		}
	}
	if rawSiteID, exists := details["site_id"]; exists {
		siteID, ok := rawSiteID.(string)
		if !ok || strings.TrimSpace(siteID) == "" || deviceID < 1 { return shared.ErrForbidden("site log has no valid resource scope") }
		if projectID > 0 {
			var linked int
			if err := s.db.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM project_sites WHERE project_id=? AND device_id=? AND site_id=?`, projectID, deviceID, siteID).Scan(&linked); err != nil { return shared.ErrInternal("could not check project log scope") }
			if linked == 0 { return shared.ErrForbidden("site log is outside this project") }
		}
		if err := s.authorizeSiteProjects(ctx, deviceID, siteID, ownerID, actorID, siteAction); err != nil { return err }
	}

	if databaseID, exists := projectLogDetailID(details, "database_id"); exists {
		if databaseID < 1 { return shared.ErrForbidden("database log has no valid resource scope") }
		resourceOwnerID, _, _, err := s.authorizeDatabaseInstance(ctx, databaseID, actorID, "database.read")
		if err != nil { return err }
		if resourceOwnerID != ownerID { return shared.ErrForbidden("database log is outside this workspace") }
		if projectID > 0 {
			var linked int
			if err := s.db.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM project_database_servers pds JOIN db_instances d ON d.server_id=pds.database_server_id WHERE pds.project_id=? AND d.id=? AND d.user_id=?`, projectID, databaseID, ownerID).Scan(&linked); err != nil { return shared.ErrInternal("could not check project database log scope") }
			if linked == 0 { return shared.ErrForbidden("database log is outside this project") }
		}
	}
	for _, key := range []string{"database_server_id"} {
		if serverID, exists := projectLogDetailID(details, key); exists {
			if serverID < 1 { return shared.ErrForbidden("database server log has no valid resource scope") }
			resourceOwnerID, _, err := s.authorizeDatabaseServer(ctx, serverID, actorID, "database.read")
			if err != nil { return err }
			if resourceOwnerID != ownerID { return shared.ErrForbidden("database server log is outside this workspace") }
			if projectID > 0 {
				var linked int
				if err := s.db.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM project_database_servers WHERE project_id=? AND database_server_id=?`, projectID, serverID).Scan(&linked); err != nil { return shared.ErrInternal("could not check project database server log scope") }
				if linked == 0 { return shared.ErrForbidden("database server log is outside this project") }
			}
		}
	}
	if backupID, exists := projectLogDetailID(details, "backup_id"); exists {
		if backupID < 1 { return shared.ErrForbidden("backup log has no valid resource scope") }
		resourceOwnerID, _, err := s.authorizeBackup(ctx, backupID, actorID, "backup.read")
		if err != nil { return err }
		if resourceOwnerID != ownerID { return shared.ErrForbidden("backup log is outside this workspace") }
	}
	return nil
}

// authorizeBackup resolves backup ownership before read, restore or transfer operations.
func (s *Server) authorizeBackup(ctx context.Context, backupID, actorID int64, action string) (ownerID, deviceID int64, err error) {
	if backupID < 1 {
		return 0, 0, shared.ErrBadRequest("invalid backup id")
	}
	var resourceType, resourceID string
	var serverID sql.NullInt64
	err = s.db.conn.QueryRowContext(ctx, "SELECT user_id,device_id,resource_type,resource_id,database_server_id FROM backups WHERE id=?", backupID).Scan(&ownerID, &deviceID, &resourceType, &resourceID, &serverID)
	if err == sql.ErrNoRows { return 0, 0, shared.ErrNotFound("backup not found") }
	if err != nil { return 0, 0, shared.ErrInternal("could not resolve backup owner") }
	switch resourceType {
	case "database":
		if !serverID.Valid || serverID.Int64 < 1 { return 0, 0, shared.ErrInternal("database backup has no owning server") }
		if err = s.authorizeDatabaseServerProjects(ctx, serverID.Int64, ownerID, actorID, action); err != nil { return 0, 0, err }
	case "database_server":
		resourceServerID := int64(0)
		if serverID.Valid {
			resourceServerID = serverID.Int64
		} else {
			resourceServerID, err = strconv.ParseInt(resourceID, 10, 64)
			if err != nil || resourceServerID < 1 { return 0, 0, shared.ErrInternal("database server backup resource is invalid") }
		}
		serverOwnerID, serverDeviceID, authErr := s.authorizeDatabaseServer(ctx, resourceServerID, actorID, action)
		if authErr != nil { return 0, 0, authErr }
		if serverOwnerID != ownerID || serverDeviceID != deviceID { return 0, 0, shared.ErrInternal("database server backup ownership is inconsistent") }
	case "site":
		parts := strings.SplitN(resourceID, ":", 2)
		if len(parts) != 2 { return 0, 0, shared.ErrInternal("site backup resource is invalid") }
		resourceDeviceID, parseErr := strconv.ParseInt(parts[0], 10, 64)
		if parseErr != nil || resourceDeviceID != deviceID || !validBackupSiteID(parts[1]) { return 0, 0, shared.ErrInternal("site backup resource is invalid") }
		if err = s.authorizeSiteProjects(ctx, deviceID, parts[1], ownerID, actorID, action); err != nil { return 0, 0, err }
	default:
		return 0, 0, shared.ErrInternal("unsupported backup resource")
	}
	return ownerID, deviceID, nil
}

func (s *Server) authorizeDeviceOwner(ctx context.Context, deviceID, actorID int64, action string) (ownerID int64, err error) {
	if deviceID < 1 {
		return 0, shared.ErrBadRequest("invalid device id")
	}
	err = s.db.conn.QueryRowContext(ctx, "SELECT user_id FROM devices WHERE id=?", deviceID).Scan(&ownerID)
	if err == sql.ErrNoRows {
		return 0, shared.ErrNotFound("device not found")
	}
	if err != nil {
		return 0, shared.ErrInternal("could not resolve device owner")
	}
	if err = s.authorizeWorkspace(ctx, ownerID, actorID, action); err != nil {
		return 0, err
	}
	return ownerID, nil
}
