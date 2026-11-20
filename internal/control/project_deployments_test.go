package control

import "testing"

func TestDeploymentStoredState(t *testing.T) {
	cases := map[string]string{
		"queued":"queued",
		"preparing":"preparing",
		"cloning":"cloning",
		"installing":"installing",
		"building":"building",
		"starting":"starting",
		"healthy":"succeeded",
		"failed":"failed",
		"cancelled":"cancelled",
		"unknown":"running",
	}
	for input, want := range cases {
		if got := deploymentStoredState(input); got != want {
			t.Errorf("deploymentStoredState(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestDeploymentTerminal(t *testing.T) {
	for _, state := range []string{"succeeded", "failed", "cancelled"} {
		if !deploymentTerminal(state) { t.Errorf("%q should be terminal", state) }
	}
	for _, state := range []string{"queued", "preparing", "building", "starting", "running"} {
		if deploymentTerminal(state) { t.Errorf("%q should not be terminal", state) }
	}
}

func TestPublicYevqoBaseURL(t *testing.T) {
	t.Setenv("YEVQO_PUBLIC_URL", "https://panel.example.test/")
	got, ok := publicYevqoBaseURL()
	if !ok || got != "https://panel.example.test" { t.Fatalf("unexpected normalized URL: %q, %v", got, ok) }
	for _, value := range []string{"http://panel.example.test", "https://user:pass@panel.example.test", "https://panel.example.test/?x=1", "not-a-url"} {
		t.Setenv("YEVQO_PUBLIC_URL", value)
		if _, ok := publicYevqoBaseURL(); ok { t.Errorf("invalid public URL accepted: %q", value) }
	}
}
