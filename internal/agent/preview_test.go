package agent

import "testing"

func TestValidPreviewID(t *testing.T) {
	for _, id := range []string{"preview_0123456789abcdef", "preview_aaaaaaaaaaaaaaaa"} {
		if !validPreviewID(id) { t.Errorf("expected valid preview ID %q", id) }
	}
	for _, id := range []string{"site_0123456789abcdef", "preview_short", "preview_../../etc", "preview_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} {
		if validPreviewID(id) { t.Errorf("expected invalid preview ID %q", id) }
	}
}
