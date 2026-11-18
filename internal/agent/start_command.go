package agent

import "os/exec"

// parseCustomStartCommand uses the same shell-free argv validation as builds.
// It accepts any installed Termux tool or an executable file inside the site.
func parseCustomStartCommand(root, raw string) (*exec.Cmd, error) {
	step, err := parseManifestBuildCommand(root, "start", raw)
	if err != nil {
		return nil, err
	}
	return exec.Command(step.exe, step.args...), nil
}
