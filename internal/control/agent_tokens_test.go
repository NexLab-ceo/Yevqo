package control

import "testing"

func TestHashAgentToken(t *testing.T) {
	first := hashAgentToken("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	second := hashAgentToken("1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if first == second { t.Fatal("different tokens must have different hashes") }
	if len(first) != 64 { t.Fatalf("expected SHA-256 hex digest, got %d chars", len(first)) }
	if first == "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" {
		t.Fatal("raw token must not be stored as its own hash")
	}
}
