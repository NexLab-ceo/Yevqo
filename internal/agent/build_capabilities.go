package agent

import "os/exec"

// BuildCapabilities describes executables available on the Agent host.
// It is a preflight only: it does not contact daemons or execute project code.
type BuildCapabilities struct {
	DockerCLI       bool   `json:"docker_cli"`
	PodmanCLI       bool   `json:"podman_cli"`
	NixpacksCLI     bool   `json:"nixpacks_cli"`
	GitCLI          bool   `json:"git_cli"`
	ContainerBackend string `json:"container_backend,omitempty"`
	CanBuildDockerfile bool `json:"can_build_dockerfile"`
	CanBuildNixpacks bool   `json:"can_build_nixpacks"`
	CustomCommandsIsolated bool `json:"custom_commands_isolated"`
	Warnings        []string `json:"warnings,omitempty"`
}

// DetectBuildCapabilities checks PATH only. Docker/Podman daemon access,
// privileges, storage and network availability must be verified at execution.
func DetectBuildCapabilities() BuildCapabilities {
	c := BuildCapabilities{
		DockerCLI:   executableAvailable("docker"),
		PodmanCLI:   executableAvailable("podman"),
		NixpacksCLI: executableAvailable("nixpacks"),
		GitCLI:      executableAvailable("git"),
		Warnings:    []string{},
	}
	switch {
	case c.DockerCLI:
		c.ContainerBackend = "docker"
	case c.PodmanCLI:
		c.ContainerBackend = "podman"
	}
	c.CanBuildDockerfile = c.ContainerBackend != ""
	c.CanBuildNixpacks = c.NixpacksCLI && c.ContainerBackend != ""
	// Never run user-supplied build commands directly on the Agent host.
	// This remains false until a resource-limited isolation backend exists.
	c.CustomCommandsIsolated = false
	if c.ContainerBackend == "" {
		c.Warnings = append(c.Warnings, "No Docker or Podman CLI found; container builds are unavailable on this Agent.")
	}
	if c.NixpacksCLI && c.ContainerBackend == "" {
		c.Warnings = append(c.Warnings, "Nixpacks is installed, but no container backend is available.")
	}
	if !c.CustomCommandsIsolated {
		c.Warnings = append(c.Warnings, "Custom build commands are not executed: an isolated build sandbox is not implemented yet.")
	}
	return c
}

func executableAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
