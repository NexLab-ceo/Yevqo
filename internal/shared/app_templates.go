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
	{ID:"nodejs-http", Name:"Node.js HTTP", Description:"Runnable Node.js HTTP server with npm start", Runtime:"nodejs", EntryPoint:"server.js"},
	{ID:"static-site", Name:"Static Website", Description:"Static HTML website served by the local PHP development server", Runtime:"static", EntryPoint:"index.html"},
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
