package control

import (
	"context"
	"database/sql"
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
