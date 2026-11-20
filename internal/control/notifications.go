package control

import (
	"context"

)

// notifyWorkspaceEvent delivers a resource event to its workspace owner and,
// when different, to the member who initiated the action.
func (s *Server) notifyWorkspaceEvent(ctx context.Context, ownerID, actorID int64, kind, title, body string) {
	recipients := []int64{ownerID}
	if actorID > 0 && actorID != ownerID {
		recipients = append(recipients, actorID)
	}
	for _, recipientID := range recipients {
		_, _ = s.db.conn.ExecContext(ctx, `INSERT INTO notifications(user_id,type,title,body) VALUES(?,?,?,?)`, recipientID, kind, title, body)
	}
}
