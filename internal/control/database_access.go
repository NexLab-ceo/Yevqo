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
