package control

import (
	"strings"
	"testing"
)

func TestSanitizeSystemLogMetadataRedactsSensitiveValuesRecursively(t *testing.T) {
	input := map[string]interface{}{
		"access_token": "ghp-secret-token",
		"safe":         "deployment failed",
		"typed_items": []map[string]interface{}{{"refresh_token": "typed-secret"}},
		"details": []interface{}{
			map[string]interface{}{
				"authorization": "Bearer secret-header",
				"cause":         "request failed with api_key=secret-api-key",
			},
			"retry with Bearer another-secret",
		},
	}

	clean, ok := sanitizeSystemLogValue(input).(map[string]interface{})
	if !ok {
		t.Fatal("sanitized metadata is not a map")
	}
	if clean["access_token"] != "[REDACTED]" {
		t.Fatalf("access token was not redacted: %#v", clean["access_token"])
	}
	if clean["safe"] != "deployment failed" {
		t.Fatalf("safe metadata was changed: %#v", clean["safe"])
	}
	typedItems, ok := clean["typed_items"].([]map[string]interface{})
	if !ok || len(typedItems) != 1 || typedItems[0]["refresh_token"] != "[REDACTED]" {
		t.Fatalf("typed nested metadata was not redacted: %#v", clean["typed_items"])
	}
	details, ok := clean["details"].([]interface{})
	if !ok || len(details) != 2 {
		t.Fatalf("unexpected nested details: %#v", clean["details"])
	}
	nested, ok := details[0].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected nested metadata: %#v", details[0])
	}
	if nested["authorization"] != "[REDACTED]" {
		t.Fatalf("authorization value was not redacted: %#v", nested["authorization"])
	}
	cause, _ := nested["cause"].(string)
	if strings.Contains(cause, "secret-api-key") || !strings.Contains(cause, "[REDACTED]") {
		t.Fatalf("API key in diagnostic string was not redacted: %q", cause)
	}
	last, _ := details[1].(string)
	if strings.Contains(last, "another-secret") || !strings.Contains(last, "Bearer [REDACTED]") {
		t.Fatalf("Bearer credential was not redacted: %q", last)
	}
}

func TestRedactSystemLogStringRedactsQueryCredentials(t *testing.T) {
	got := redactSystemLogString("/callback?access_token=secret-value&state=safe-state")
	if strings.Contains(got, "secret-value") {
		t.Fatalf("query credential leaked: %q", got)
	}
	if !strings.Contains(got, "state=safe-state") {
		t.Fatalf("non-sensitive query value should be preserved: %q", got)
	}
}
