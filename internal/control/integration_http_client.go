package control

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// integrationHTTPClient uses Go's resolver with the DNS servers configured for
// the current runtime. Android/Termux does not expose /etc/resolv.conf at the
// usual Linux path; its resolver configuration lives under $PREFIX/etc.
var integrationHTTPClient = newIntegrationHTTPClient()

func newIntegrationHTTPClient() *http.Client {
	var nextServer atomic.Uint32
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			servers := integrationDNSServers()
			if len(servers) > 0 {
				server := servers[int(nextServer.Add(1)-1)%len(servers)]
				address = net.JoinHostPort(server, "53")
			}
			return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, address)
		},
	}
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Resolver:  resolver,
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		ForceAttemptHTTP2:     true,
	}
	return &http.Client{Transport: transport, Timeout: 20 * time.Second}
}

// integrationDNSServers reads the resolver configuration used by the active
// runtime. Prefer Termux's private PREFIX path when running inside Termux.
func integrationDNSServers() []string {
	prefix := strings.TrimSpace(os.Getenv("PREFIX"))
	if prefix != "" {
		return integrationDNSServersFrom(prefix+"/etc/resolv.conf", "/etc/resolv.conf")
	}
	return integrationDNSServersFrom("/etc/resolv.conf")
}

// integrationDNSServersFrom returns the first usable resolver configuration.
// Keeping path selection separate makes Termux precedence and Linux fallback testable.
func integrationDNSServersFrom(paths ...string) []string {
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		servers := parseIntegrationResolverConfig(file)
		_ = file.Close()
		if len(servers) > 0 {
			return servers
		}
	}
	return nil
}

func parseIntegrationResolverConfig(file io.Reader) []string {
	var servers []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(strings.SplitN(scanner.Text(), "#", 2)[0])
		if len(fields) < 2 || fields[0] != "nameserver" {
			continue
		}
		ip := net.ParseIP(fields[1])
		if ip == nil {
			continue
		}
		servers = append(servers, ip.String())
	}
	return servers
}
