package control

import (
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
