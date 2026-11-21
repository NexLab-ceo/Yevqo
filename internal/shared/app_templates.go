package shared

// AppTemplate is a first-party runnable starter exposed by the Control Plane
// and consumed by the Agent when creating a site.
type AppTemplate struct {
	ID string `json:"id"`
	Name string `json:"name"`
	Description string `json:"description"`
	Runtime string `json:"runtime"`
	EntryPoint string `json:"entry_point"`
}

var firstPartyAppTemplates = []AppTemplate{
	{ID:"php-basic", Name:"PHP Website", Description:"PHP starter served by the local PHP runtime", Runtime:"php", EntryPoint:"index.php"},
	{ID:"php-api", Name:"PHP JSON API", Description:"Minimal JSON API endpoint using the local PHP runtime", Runtime:"php", EntryPoint:"index.php"},
	{ID:"nodejs-http", Name:"Node.js HTTP", Description:"Runnable Node.js HTTP server with npm start", Runtime:"nodejs", EntryPoint:"server.js"},
	{ID:"nodejs-api", Name:"Node.js JSON API", Description:"Node.js JSON API with a built-in health endpoint", Runtime:"nodejs", EntryPoint:"server.js"},
	{ID:"nextjs-app", Name:"Next.js App", Description:"Next.js App Router starter with a production build and health route", Runtime:"nodejs", EntryPoint:"app/page.js"},
	{ID:"static-site", Name:"Static Website", Description:"Static HTML website served by the local PHP development server", Runtime:"static", EntryPoint:"index.html"},
	{ID:"go-http", Name:"Go HTTP", Description:"Go HTTP starter with a standard-library server; build before starting", Runtime:"go", EntryPoint:"main.go"},
	{ID:"custom-app", Name:"Custom Language App", Description:"Blank project for any installed Termux toolchain; configure build and start commands", Runtime:"custom", EntryPoint:""},
}

// ListAppTemplates returns a defensive copy of the built-in template catalog.
func ListAppTemplates() []AppTemplate {
	out := make([]AppTemplate, len(firstPartyAppTemplates))
	copy(out, firstPartyAppTemplates)
	return out
}

// FindAppTemplate returns a template by its stable identifier.
func FindAppTemplate(id string) (AppTemplate, bool) {
	for _, template := range firstPartyAppTemplates {
		if template.ID == id { return template, true }
	}
	return AppTemplate{}, false
}
