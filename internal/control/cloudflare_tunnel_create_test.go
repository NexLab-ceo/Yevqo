package control

import (
	"strings"
	"testing"
)

func TestCloudflareTunnelName(t *testing.T) {
	name := cloudflareTunnelName(42, "My App / Production")
	if name != "yevqo-My-App-Production-42" {
		t.Fatalf("unexpected tunnel name %q", name)
	}
	if len(cloudflareTunnelName(123456, strings.Repeat("a", 100))) > 64 {
		t.Fatal("tunnel name exceeds Cloudflare's limit")
	}
	if got := cloudflareTunnelName(1, "///"); got != "yevqo-site-1" {
		t.Fatalf("unexpected fallback name %q", got)
	}
}
