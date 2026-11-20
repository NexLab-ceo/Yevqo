package control

import (
	"context"
	"database/sql"

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
	if err = s.authorizeWorkspace(ctx, ownerID, actorID, action); err != nil {
		return 0, 0, err
	}
	return ownerID, deviceID, nil
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
	if err = s.authorizeWorkspace(ctx, ownerID, actorID, action); err != nil {
		return 0, 0, 0, err
	}
	return ownerID, serverID, deviceID, nil
}

// authorizeDeviceOwner resolves the workspace owner for a device-scoped
// resource and applies the requested role permission.
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
