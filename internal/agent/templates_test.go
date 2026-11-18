package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAppTemplateCatalogAndScaffolding(t *testing.T) {
	templates := ListAppTemplates()
	if len(templates) != 3 {
		t.Fatalf("template catalog has %d entries, want 3", len(templates))
	}
	want := map[string]string{
		"php-basic":"index.php",
		"nodejs-http":"server.js",
		"static-site":"index.html",
	}
	for _, template := range templates {
		entry, ok := want[template.ID]
		if !ok { t.Fatalf("unexpected template %q", template.ID) }
		if template.EntryPoint != entry { t.Fatalf("%s entry point = %q, want %q", template.ID, template.EntryPoint, entry) }
		root := t.TempDir()
		manager := NewProcessManager(root)
		if err := manager.CreateSiteFromTemplate("template-demo", 8123, template.ID, template.Runtime); err != nil {
			t.Fatalf("create %s: %v", template.ID, err)
		}
		siteRoot := filepath.Join(root, "sites", "template-demo")
		if _, err := os.Stat(filepath.Join(siteRoot, entry)); err != nil {
			t.Fatalf("%s entry point was not generated: %v", template.ID, err)
		}
		configBytes, err := os.ReadFile(filepath.Join(siteRoot, "yevqo.template.json"))
		if err != nil { t.Fatalf("%s template config missing: %v", template.ID, err) }
		var config SiteTemplateConfig
		if err := json.Unmarshal(configBytes, &config); err != nil { t.Fatalf("%s invalid template config: %v", template.ID, err) }
		if config.TemplateID != template.ID || config.Runtime != template.Runtime || config.Port != 8123 || config.EntryPoint != entry {
			t.Fatalf("%s generated incorrect config: %#v", template.ID, config)
		}
		if template.ID == "nodejs-http" && config.StartCommand != "npm run start" {
			t.Fatalf("Node.js launch command = %q", config.StartCommand)
		}
	}
}

func TestCreateSiteFromTemplateRejectsUnknownAndMismatchedTemplates(t *testing.T) {
	manager := NewProcessManager(t.TempDir())
	if err := manager.CreateSiteFromTemplate("bad-template", 8123, "docker-app", "nodejs"); err == nil {
		t.Fatal("unknown template was accepted")
	}
	if err := manager.CreateSiteFromTemplate("bad-runtime", 8123, "php-basic", "static"); err == nil {
		t.Fatal("template/runtime mismatch was accepted")
	}
	if _, err := os.Stat(filepath.Join(manager.DataRoot, "sites")); !os.IsNotExist(err) {
		t.Fatalf("invalid template request created site files: %v", err)
	}
}
