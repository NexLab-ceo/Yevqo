package control

import (
	"net/http/httptest"
	"testing"
)

func TestClientIPTrustsCloudflareHeaderOnlyFromLoopback(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		cfIP       string
		xff        string
		want       string
	}{
		{
			name:       "IPv4 loopback accepts Cloudflare client address",
			remoteAddr: "127.0.0.1:41234",
			cfIP:       "203.0.113.8",
			xff:        "198.51.100.9",
			want:       "203.0.113.8",
		},
		{
			name:       "IPv6 loopback accepts Cloudflare client address",
			remoteAddr: "[::1]:41234",
			cfIP:       "2001:db8::8",
			want:       "2001:db8::8",
		},
		{
			name:       "remote peer cannot spoof Cloudflare header",
			remoteAddr: "198.51.100.20:41234",
			cfIP:       "203.0.113.8",
			xff:        "192.0.2.4",
			want:       "198.51.100.20",
		},
		{
			name:       "remote peer cannot spoof forwarded-for",
			remoteAddr: "198.51.100.20:41234",
			xff:        "203.0.113.8",
			want:       "198.51.100.20",
		},
		{
			name:       "invalid Cloudflare header falls back to loopback peer",
			remoteAddr: "127.0.0.1:41234",
			cfIP:       "not-an-ip",
			want:       "127.0.0.1",
		},
		{
			name:       "invalid remote address is unknown",
			remoteAddr: "not-an-address",
			cfIP:       "203.0.113.8",
			want:       "unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://localhost/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.cfIP != "" {
				req.Header.Set("CF-Connecting-IP", tt.cfIP)
			}
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if got := clientIP(req); got != tt.want {
				t.Fatalf("clientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
