package control

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestReadProviderJSONIncludesSafeProviderDiagnostics(t *testing.T) {
	body := `{"message":"Resource not accessible by integration","token":"must-not-leak"}`
	resp := &http.Response{
		StatusCode: http.StatusForbidden,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	resp.Header.Set("X-GitHub-Request-Id", "ABC123")
	resp.Header.Set("Retry-After", "30")

	err := readProviderJSON(resp, &struct{}{})
	if err == nil {
		t.Fatal("expected provider error")
	}
	message := err.Error()
	for _, expected := range []string{
		"provider returned HTTP 403",
		"Resource not accessible by integration",
		"request_id=ABC123",
		"retry_after=30",
	} {
		if !strings.Contains(message, expected) {
			t.Fatalf("error %q does not contain %q", message, expected)
		}
	}
	if strings.Contains(message, "must-not-leak") {
		t.Fatalf("provider error exposed an untrusted response field: %q", message)
	}
}

func TestReadProviderJSONDecodesSuccessfulResponse(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"login":"yevqo"}`)),
	}
	var result struct {
		Login string `json:"login"`
	}
	if err := readProviderJSON(resp, &result); err != nil {
		t.Fatalf("readProviderJSON returned error: %v", err)
	}
	if result.Login != "yevqo" {
		t.Fatalf("unexpected decoded login %q", result.Login)
	}
}
