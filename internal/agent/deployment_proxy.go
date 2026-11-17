package agent

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"time"
)

// deploymentProxy owns the stable site listener. Its target is changed with a
// single atomic pointer swap, so in-flight requests may finish on the old
// release while new requests are routed to the candidate.
type deploymentProxy struct {
	server   *http.Server
	listener net.Listener
	target   atomic.Pointer[url.URL]
}

func newDeploymentProxy(publicPort, backendPort int) (*deploymentProxy, error) {
	if publicPort < 1024 || publicPort > 65535 || backendPort < 1024 || backendPort > 65535 || publicPort == backendPort {
		return nil, fmt.Errorf("invalid deployment proxy ports")
	}
	target, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", backendPort))
	if err != nil {
		return nil, fmt.Errorf("parse deployment backend: %w", err)
	}
	proxy := &deploymentProxy{}
	proxy.target.Store(target)
	reverse := httputil.NewSingleHostReverseProxy(target)
	reverse.Director = func(req *http.Request) {
		current := proxy.target.Load()
		if current == nil {
			return
		}
		req.URL.Scheme = current.Scheme
		req.URL.Host = current.Host
		req.Host = current.Host
	}
	reverse.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		http.Error(w, "site backend temporarily unavailable", http.StatusBadGateway)
	}
	proxy.server = &http.Server{
		Handler:           reverse,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", publicPort))
	if err != nil {
		return nil, fmt.Errorf("bind stable site listener on port %d: %w", publicPort, err)
	}
	proxy.listener = listener
	go func() { _ = proxy.server.Serve(listener) }()
	return proxy, nil
}

func (p *deploymentProxy) switchBackend(backendPort int) error {
	if p == nil || backendPort < 1024 || backendPort > 65535 {
		return fmt.Errorf("invalid deployment proxy backend")
	}
	target, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", backendPort))
	if err != nil {
		return err
	}
	p.target.Store(target)
	return nil
}

func (p *deploymentProxy) close() error {
	if p == nil || p.server == nil {
		return nil
	}
	return p.server.Close()
}
