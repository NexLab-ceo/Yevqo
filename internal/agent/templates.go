package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"veltrix/internal/shared"
)

// ListAppTemplates exposes the shared catalog to Agent callers and tests.
func ListAppTemplates() []shared.AppTemplate { return shared.ListAppTemplates() }

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

// CreateSiteFromTemplate scaffolds a supported template and persists its
// launch configuration. An empty template ID keeps compatibility with older
// Agents by resolving the template from the existing runtime field.
func (m *ProcessManager) CreateSiteFromTemplate(siteID string, port int, templateID, runtime string) error {
	if templateID == "" {
		switch runtime {
		case "php": templateID = "php-basic"
		case "nodejs": templateID = "nodejs-http"
		case "static": templateID = "static-site"
		default: return fmt.Errorf("no starter template is available for runtime %q", runtime)
		}
	}
	selectedValue, found := shared.FindAppTemplate(templateID)
	if !found {
		return fmt.Errorf("unknown app template %q", templateID)
	}
	if selectedValue.Runtime != runtime {
		return fmt.Errorf("template %q requires runtime %q, got %q", templateID, selectedValue.Runtime, runtime)
	}
	if err := m.CreateSite(siteID, port, runtime); err != nil {
		return err
	}
	config := SiteTemplateConfig{
		SchemaVersion: 1,
		TemplateID: selectedValue.ID,
		Runtime: selectedValue.Runtime,
		Port: port,
		EntryPoint: selectedValue.EntryPoint,
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
	_, ok := shared.FindAppTemplate(templateID)
	return ok
}
