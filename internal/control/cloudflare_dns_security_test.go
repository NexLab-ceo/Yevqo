package control

import (
	"strings"
	"testing"
)

func TestCloudflareDNSRecordIDValidation(t *testing.T) {
	valid := strings.Repeat("a", 32)
	if !validCloudflareDNSRecordID(valid) {
		t.Fatal("valid Cloudflare DNS record ID was rejected")
	}
	for _, value := range []string{
		"", "short", strings.Repeat("g", 32), strings.Repeat("a", 31),
		strings.Repeat("a", 33), strings.Repeat("a", 16) + "/../../users",
		strings.Repeat("a", 16) + "?page=2",
	} {
		if validCloudflareDNSRecordID(value) {
			t.Fatalf("invalid DNS record ID %q was accepted", value)
		}
	}
}

func TestCloudflareHostnameZoneBoundary(t *testing.T) {
	for _, pair := range [][2]string{
		{"example.com", "example.com"},
		{"app.example.com", "example.com"},
		{"a.b.example.com", "example.com"},
		{"APP.EXAMPLE.COM.", "example.com"},
	} {
		if !hostnameBelongsToZone(pair[0], pair[1]) {
			t.Errorf("%q should belong to %q", pair[0], pair[1])
		}
	}
	for _, host := range []string{"evil-example.com", "example.com.evil.org", "example.com.", "other.net", ""} {
		if hostnameBelongsToZone(host, "example.com") && host != "example.com." {
			t.Errorf("%q must not belong to example.com", host)
		}
	}
}
