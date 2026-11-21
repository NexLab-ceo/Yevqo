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
		case "go": templateID = "go-http"
		case "custom": templateID = "custom-app"
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
	// Replace only the known entrypoint in the freshly created directory.
	// Template content is first-party and never interpreted as a command.
	var starter string
	switch selectedValue.ID {
	case "php-api":
		starter = "<?php\nheader('Content-Type: application/json; charset=utf-8');\necho json_encode(['ok' => true, 'service' => 'yevqo', 'message' => 'Hello from Yevqo API'], JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);\n"
	case "nodejs-api":
		starter = "const http = require('http');\nconst host = process.env.HOST || '127.0.0.1';\nconst port = Number(process.env.PORT || 3000);\nhttp.createServer((req, res) => {\n  res.setHeader('content-type', 'application/json; charset=utf-8');\n  if (req.url === '/health') { res.end(JSON.stringify({ ok: true, status: 'healthy' })); return; }\n  res.end(JSON.stringify({ ok: true, service: 'yevqo', message: 'Hello from Yevqo API' }));\n}).listen(port, host);\n"
	}
	if selectedValue.ID == "nextjs-app" {
		root := filepath.Join(m.DataRoot, "sites", siteID)
		files := map[string]string{
			"package.json": "{\\n  \\"name\\": \\"yevqo-nextjs-app\\",\\n  \\"version\\": \\"1.0.0\\",\\n  \\"private\\": true,\\n  \\"scripts\\": { \\"dev\\": \\"next dev\\", \\"build\\": \\"next build\\", \\"start\\": \\"next start -H 127.0.0.1\\" },\\n  \\"dependencies\\": { \\"next\\": \\"^15.5.6\\", \\"react\\": \\"^19.1.1\\", \\"react-dom\\": \\"^19.1.1\\" }\\n}\\n",
			"app/page.js": "export default function Home() {\\n  return (\\n    <main style={{ maxWidth: 720, margin: '15vh auto', padding: 24, fontFamily: 'system-ui' }}>\\n      <p style={{ color: '#39834a', fontWeight: 700 }}>YEVQO · NEXT.JS</p>\\n      <h1>Your Next.js app is ready.</h1>\\n      <p>Edit app/page.js to start building your project.</p>\\n      <a href='/api/health'>Check application health</a>\\n    </main>\\n  );\\n}\\n",
			"app/layout.js": "export const metadata = { title: 'Yevqo Next.js App', description: 'A Next.js application hosted with Yevqo' };\\n\\nexport default function RootLayout({ children }) {\\n  return <html lang='en'><body>{children}</body></html>;\\n}\\n",
			"app/api/health/route.js": "export async function GET() {\\n  return Response.json({ ok: true, status: 'healthy' });\\n}\\n",
			"next.config.mjs": "/** @type {import('next').NextConfig} */\\nconst nextConfig = {\\n  poweredByHeader: false,\\n};\\n\\nexport default nextConfig;\\n",
			".gitignore": "node_modules/\\n.next/\\nout/\\n.env*\\n!.env.example\\n",
		}
		for name, content := range files {
			path := filepath.Join(root, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil { return fmt.Errorf("create %s directory: %w", name, err) }
			if err := os.WriteFile(path, []byte(content), 0o640); err != nil { return fmt.Errorf("write %s starter: %w", name, err) }
		}
	}
	if starter != "" {
		root := filepath.Join(m.DataRoot, "sites", siteID)
		if err := os.WriteFile(filepath.Join(root, selectedValue.EntryPoint), []byte(starter), 0o640); err != nil {
			return fmt.Errorf("write %s starter: %w", selectedValue.ID, err)
		}
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
