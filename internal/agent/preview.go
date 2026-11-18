package agent

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"veltrix/internal/shared"
)

// DeployPreview creates a completely separate site namespace. The synthetic
// preview site ID gives it its own directory, process registry entry, logs,
// release pointer and port; production site state is never touched.
func (m *ProcessManager) DeployPreview(req shared.PreviewDeploy) (int, string, string, string, error) {
	if runtime.GOOS != "android" { return 0, "", "", "", fmt.Errorf("Preview Deployments are supported only on Android/Termux") }
	if !validPreviewID(req.PreviewID) {
		return 0, "", "", "", fmt.Errorf("invalid preview id")
	}
	req.SiteAction.SiteId = req.PreviewID
	req.SiteAction.Path = "sites/" + req.PreviewID
	if req.Port < 1024 || req.Port > 65535 {
		return 0, "", "", "", fmt.Errorf("preview port must be between 1024 and 65535")
	}
	return m.DeployGitSiteWithJob(context.Background(), "preview-"+req.PreviewID, req.SiteAction, false)
}

// RemovePreview only accepts IDs from the reserved preview namespace.
func (m *ProcessManager) RemovePreview(previewID string) error {
	if runtime.GOOS != "android" { return fmt.Errorf("Preview Deployments are supported only on Android/Termux") }
	if !validPreviewID(previewID) {
		return fmt.Errorf("invalid preview id")
	}
	m.operationMu.Lock()
	defer m.operationMu.Unlock()

	// A stopped preview has no process record; that is not an error for deletion.
	if err := m.stopSiteLocked(previewID); err != nil && !strings.Contains(err.Error(), "is not running") {
		return err
	}
	dir, err := safeSiteDirectory(m.DataRoot, previewID)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove preview directory: %w", err)
	}
	return nil
}

func validPreviewID(id string) bool {
	return strings.HasPrefix(id, "preview_") && len(id) >= 16 && len(id) <= 64 && validSiteID(id)
}
