package agent

import (
	"fmt"
	"runtime"
	"strings"
)

// BuildProvider describes where and how a build can safely execute.
type BuildProvider struct {
	ID string `json:"id"`
	Name string `json:"name"`
	RequiresLinux bool `json:"requires_linux"`
	RequiresExecutor bool `json:"requires_executor"`
	Available bool `json:"available"`
	Reason string `json:"reason,omitempty"`
}

var buildProviderCatalog = []BuildProvider{
	{ID:"android-local", Name:"Android local", Available:true},
	{ID:"static", Name:"Static files", Available:true},
	{ID:"dockerfile", Name:"Dockerfile", RequiresLinux:true, RequiresExecutor:true},
	{ID:"nixpacks", Name:"Nixpacks", RequiresLinux:true, RequiresExecutor:true},
	{ID:"railpack", Name:"Railpack", RequiresLinux:true, RequiresExecutor:true},
}

// ListBuildProviders returns a copy of the supported provider catalog.
func ListBuildProviders() []BuildProvider {
	out := make([]BuildProvider, len(buildProviderCatalog))
	copy(out, buildProviderCatalog)
	return out
}

// ResolveBuildProvider validates the requested provider against the actual
// execution environment. It never silently falls back from a remote build to
// an Android shell.
func ResolveBuildProvider(requested string, android AndroidBuildEnvironment, linuxExecutorAvailable bool) (BuildProvider, error) {
	id := strings.ToLower(strings.TrimSpace(requested))
	if id == "" || id == "auto" { id = "android-local" }
	for _, provider := range buildProviderCatalog {
		if provider.ID != id { continue }
		resolved := provider
		if provider.RequiresExecutor {
			if android.Android || android.Termux || runtime.GOOS == "android" {
				resolved.Available = false
				resolved.Reason = "Dockerfile, Nixpacks and Railpack builds require a registered remote Linux executor; Android/Termux is not a Docker host."
				return resolved, fmt.Errorf("%s: %s", provider.Name, resolved.Reason)
			}
			if !linuxExecutorAvailable {
				resolved.Available = false
				resolved.Reason = "no online Linux build executor is registered"
				return resolved, fmt.Errorf("%s: %s", provider.Name, resolved.Reason)
			}
			resolved.Available = true
		}
		return resolved, nil
	}
	return BuildProvider{}, fmt.Errorf("unsupported build provider %q", requested)
}
