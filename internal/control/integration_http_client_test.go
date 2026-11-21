package control

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseIntegrationResolverConfig(t *testing.T) {
	config := strings.NewReader(`# generated resolver configuration
search example.test
nameserver 8.8.8.8
nameserver 2001:4860:4860::8888
nameserver not-an-ip
options edns0
`)
	got := parseIntegrationResolverConfig(config)
	want := []string{"8.8.8.8", "2001:4860:4860::8888"}
	if len(got) != len(want) {
		t.Fatalf("servers = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("servers = %#v, want %#v", got, want)
		}
	}
}

func TestParseIntegrationResolverConfigIgnoresEmptyAndInvalidConfig(t *testing.T) {
	for _, config := range []string{
		"",
		"search example.test\noptions edns0\n",
		"nameserver localhost\nnameserver 999.999.999.999\n",
	} {
		if got := parseIntegrationResolverConfig(strings.NewReader(config)); len(got) != 0 {
			t.Fatalf("config %q produced unexpected DNS servers: %#v", config, got)
		}
	}
}

func TestIntegrationDNSServersPreferTermuxPrefix(t *testing.T) {
	root := t.TempDir()
	prefix := filepath.Join(root, "termux")
	if err := os.MkdirAll(filepath.Join(prefix, "etc"), 0o700); err != nil { t.Fatal(err) }
	termuxConfig := filepath.Join(prefix, "etc", "resolv.conf")
	if err := os.WriteFile(termuxConfig, []byte("nameserver 8.8.8.8\n"), 0o600); err != nil { t.Fatal(err) }
	fallback := filepath.Join(root, "system-resolv.conf")
	if err := os.WriteFile(fallback, []byte("nameserver 1.1.1.1\n"), 0o600); err != nil { t.Fatal(err) }
	got := integrationDNSServersFrom(termuxConfig, fallback)
	if len(got) != 1 || got[0] != "8.8.8.8" { t.Fatalf("servers = %#v, want Termux resolver", got) }
}

func TestIntegrationDNSServersFallBackWhenTermuxConfigIsUnavailable(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing", "resolv.conf")
	fallback := filepath.Join(root, "system-resolv.conf")
	if err := os.WriteFile(fallback, []byte("nameserver 1.1.1.1\n"), 0o600); err != nil { t.Fatal(err) }
	got := integrationDNSServersFrom(missing, fallback)
	if len(got) != 1 || got[0] != "1.1.1.1" { t.Fatalf("servers = %#v, want fallback resolver", got) }
}

func TestIntegrationDNSServersSkipInvalidTermuxConfig(t *testing.T) {
	root := t.TempDir()
	invalid := filepath.Join(root, "invalid-resolv.conf")
	fallback := filepath.Join(root, "system-resolv.conf")
	if err := os.WriteFile(invalid, []byte("nameserver localhost\n"), 0o600); err != nil { t.Fatal(err) }
	if err := os.WriteFile(fallback, []byte("nameserver 9.9.9.9\n"), 0o600); err != nil { t.Fatal(err) }
	got := integrationDNSServersFrom(invalid, fallback)
	if len(got) != 1 || got[0] != "9.9.9.9" { t.Fatalf("servers = %#v, want fallback after invalid config", got) }
}
