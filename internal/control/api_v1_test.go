package control

import "testing"

func TestAPIKeyDigestIsDeterministicAndNotPlaintext(t *testing.T) {
	token := "yevqo_live_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	first := apiKeyDigest(token)
	second := apiKeyDigest(token)
	if first != second { t.Fatal("same API key produced different digests") }
	if first == token || len(first) != 64 { t.Fatalf("unexpected API key digest: %q", first) }
	if apiKeyDigest(token+"x") == first { t.Fatal("different API keys produced the same digest") }
}
