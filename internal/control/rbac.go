package control

import "strings"

// CanWorkspaceAction is the server-side role policy. UI visibility is never
// treated as authorization.
func CanWorkspaceAction(role, action string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "owner", "admin":
		return true
	case "developer":
		switch action {
		case "device.read", "device.metrics",
			"create_site", "start_site", "stop_site", "restart", "deploy", "rebuild",
			"git_pull", "get_logs", "get_metrics", "get_site_metrics", "analyze_site_build",
			"build_site", "list_site_releases", "rollback_site_release", "cancel_deployment_job":
			return true
		}
	}
	return false
}
