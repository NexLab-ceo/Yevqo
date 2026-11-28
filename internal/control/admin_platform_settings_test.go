package control

import "testing"

func TestNormalizePlatformURL(t *testing.T) {
    valid := []string{"https://app.yevqo.cc.cd", "http://localhost:8080", "http://127.0.0.1:8080/"}
    for _, input := range valid {
        if _, err := normalizePlatformURL(input); err != nil { t.Errorf("expected %q to be valid: %v", input, err) }
    }
    invalid := []string{"", "app.yevqo.cc.cd", "http://example.com", "https://example.com/path", "https://example.com?x=1", "https://user@example.com", "https://example.com/#fragment"}
    for _, input := range invalid {
        if _, err := normalizePlatformURL(input); err == nil { t.Errorf("expected %q to be rejected", input) }
    }
}
