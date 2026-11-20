package control

import (
	"context"
	"database/sql"
		"strings"

	"veltrix/internal/shared"
)

// CanWorkspaceAction is the server-side role policy. UI visibility is never
// treated as authorization.
func CanWorkspaceAction(role, action string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "owner", "admin":
		return true
	case "developer":
		switch action {
		case "project.read", "environment.read", "device.read", "device.metrics",
			"create_site", "start_site", "stop_site", "restart", "deploy", "rebuild", "stop", "rollback",
			"git_pull", "get_logs", "get_metrics", "get_site_metrics", "analyze_site_build",
			"build_site", "cancel_build", "list_build_history", "get_build_history", "preview.deploy", "preview.delete", "list_site_releases", "rollback_site_release", "cancel_deployment_job":
			return true
		}
	}
	return false
}

func (s *Server) authorizeWorkspace(ctx context.Context, ownerID, actorID int64, action string) error {
	role, allowed, err := s.db.WorkspaceRole(ctx, ownerID, actorID)
	if err != nil { return err }
	if !allowed || !CanWorkspaceAction(role, action) {
		return shared.ErrForbidden("workspace permission denied")
	}
	return nil
}


// authorizeProjectAction resolves the owning workspace and enforces its role.
// Projects remain owned by the workspace owner; active members receive only
// the actions allowed by their assigned role.
func (s *Server) authorizeProjectOwner(ctx context.Context, projectID, actorID int64, action string) (int64, error) {
	if projectID < 1 {
		return 0, shared.ErrBadRequest("invalid project id")
	}
	var ownerID int64
	if err := s.db.conn.QueryRowContext(ctx, "SELECT user_id FROM projects WHERE id=?", projectID).Scan(&ownerID); err != nil {
		if err == sql.ErrNoRows { return 0, shared.ErrNotFound("project not found") }
		return 0, shared.ErrInternal("could not resolve project owner")
	}
	if err := s.authorizeWorkspace(ctx, ownerID, actorID, action); err != nil {
		return 0, err
	}
	return ownerID, nil
}

func (s *Server) authorizeProjectAction(ctx context.Context, projectID, actorID int64, action string) error {
	_, err := s.authorizeProjectOwner(ctx, projectID, actorID, action)
	return err
}
