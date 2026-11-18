package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AppTemplate describes a runnable first-party starter template.
type AppTemplate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Runtime     string `json:"runtime"`
	EntryPoint  string `json:"entry_point"`
}

// SiteTemplateConfig is persisted with each generated site so the selected
// template and launch contract remain inspectable and reproducible.
type SiteTemplateConfig struct {
	SchemaVersion int    `json:"schema_version"`
	TemplateID    string `json:"template_id"`
	Runtime       string `json:"runtime"`
	Port          int    `json:"port"`
	EntryPoint    string `json:"entry_point"`
	StartCommand  string `json:"start_command,omitempty"`
}

var appTemplates = []AppTemplate{
	{ID:"php-basic", Name:"PHP Website", Description:"PHP starter served by the local PHP runtime", Runtime:"php", EntryPoint:"index.php"},
	{ID:"nodejs-http", Name:"Node.js HTTP", Description:"Runnable Node.js HTTP server with npm start", Runtime:"nodejs", EntryPoint:"server.js"},
	{ID:"static-site", Name:"Static Website", Description:"Static HTML website served by the local PHP development server", Runtime:"static", EntryPoint:"index.html"},
}

// ListAppTemplates returns a defensive copy of the supported starter catalog.
func ListAppTemplates() []AppTemplate {
	out := make([]AppTemplate, len(appTemplates))
	copy(out, appTemplates)
	return out
}

// CreateSiteFromTemplate scaffolds a supported template and persists its
// launch configuration. An empty template ID keeps compatibility with older
// Agents by resolving the template from the existing runtime field.
func (m *ProcessManager) CreateSiteFromTemplate(siteID string, port int, templateID, runtime string) error {
	if templateID == "" {
		switch runtime {
		case "php": templateID = "php-basic"
		case "nodejs": templateID = "nodejs-http"
		case "static": templateID = "static-site"
		case "go":
			// Preserve the existing Go starter flow; Go is not part of the
			// requested PHP/Node/Static template catalog.
			return m.CreateSite(siteID, port, runtime)
		default: return fmt.Errorf("no starter template is available for runtime %q", runtime)
		}
	}
	var selected *AppTemplate
	for i := range appTemplates {
		if appTemplates[i].ID == templateID {
			selected = &appTemplates[i]
			break
		}
	}
	if selected == nil {
		return fmt.Errorf("unknown app template %q", templateID)
	}
	if selected.Runtime != runtime {
		return fmt.Errorf("template %q requires runtime %q, got %q", templateID, selected.Runtime, runtime)
	}
	if err := m.CreateSite(siteID, port, runtime); err != nil {
		return err
	}
	config := SiteTemplateConfig{
		SchemaVersion: 1,
		TemplateID: selected.ID,
		Runtime: selected.Runtime,
		Port: port,
		EntryPoint: selected.EntryPoint,
	}
	if runtime == "nodejs" { config.StartCommand = "npm run start" }
	encoded, err := json.MarshalIndent(config, "", "  ")
	if err != nil { return fmt.Errorf("encode template configuration: %w", err) }
	encoded = append(encoded, '\n')
	root := filepath.Join(m.DataRoot, "sites", siteID)
	if err := os.WriteFile(filepath.Join(root, "yevqo.template.json"), encoded, 0o640); err != nil {
		return fmt.Errorf("write generated template configuration: %w", err)
	}
	return nil
}

// ValidateTemplateID is shared by UI/API adapters that need a stable catalog
// validation without creating a site.
func ValidateTemplateID(templateID string) bool {
	for _, template := range appTemplates {
		if strings.EqualFold(template.ID, templateID) { return true }
	}
	return false
}
