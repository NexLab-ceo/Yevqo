package agent

import (
	"context"
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
	reverse := &httputil.ReverseProxy{
		Rewrite: func(request *httputil.ProxyRequest) {
			current := proxy.target.Load()
			if current == nil { return }
			request.SetURL(current)
			request.SetXForwarded()
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			http.Error(w, "site backend temporarily unavailable", http.StatusBadGateway)
		},
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

func allocateDeploymentBackendPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil { return 0, fmt.Errorf("allocate deployment backend port: %w", err) }
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil { return 0, fmt.Errorf("release allocated backend port: %w", err) }
	return port, nil
}

func (m *ProcessManager) ensureDeploymentProxy(siteID string, publicPort, backendPort int) error {
	if !validSiteID(siteID) { return fmt.Errorf("invalid site id for deployment proxy") }
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.proxies == nil { m.proxies = make(map[string]*deploymentProxy) }
	if existing := m.proxies[siteID]; existing != nil {
		return existing.switchBackend(backendPort)
	}
	proxy, err := newDeploymentProxy(publicPort, backendPort)
	if err != nil { return err }
	m.proxies[siteID] = proxy
	return nil
}

func (m *ProcessManager) switchDeploymentProxy(siteID string, backendPort int) error {
	m.mu.RLock()
	proxy := m.proxies[siteID]
	m.mu.RUnlock()
	if proxy == nil { return fmt.Errorf("stable deployment proxy is not running for site %s", siteID) }
	return proxy.switchBackend(backendPort)
}

func (m *ProcessManager) closeDeploymentProxy(siteID string) error {
	m.mu.Lock()
	proxy := m.proxies[siteID]
	delete(m.proxies, siteID)
	m.mu.Unlock()
	if proxy == nil { return nil }
	return proxy.close()
}

func (m *ProcessManager) stopDeploymentProcess(record ProcessRecord) error {
	if !verifyProcessRecord(record) { return nil }
	return killProcessGroup(record.PID, record.ProcessGroup)
}

// activateBlueGreenRelease starts and verifies a candidate on a private backend,
// then atomically changes the stable listener. Legacy direct-port sites are
// migrated on their first deployment; subsequent deployments keep the listener
// open throughout the switch.
func (m *ProcessManager) activateBlueGreenRelease(ctx context.Context, tx deploymentTransaction, runPath string, publicPort int, runtime string, env map[string]string, startCommand, healthPath string) (int, error) {
	if err := m.createDeploymentTransaction(tx); err != nil { return 0, fmt.Errorf("persist deployment recovery journal: %w", err) }
	backendPort, err := allocateDeploymentBackendPort()
	if err != nil { _ = m.removeDeploymentTransaction(tx.SiteID); return 0, err }

	var previousProcess *Process
	m.mu.Lock()
	previousProcess = m.procs[tx.SiteID]
	delete(m.procs, tx.SiteID)
	m.mu.Unlock()

	pid, startErr := m.startSiteWithCommandLocked(tx.SiteID, runPath, backendPort, runtime, env, startCommand)
	if startErr != nil {
		if previousProcess != nil { m.mu.Lock(); m.procs[tx.SiteID] = previousProcess; m.mu.Unlock() }
		if tx.HadPrevious {
			if recoveryErr := m.rollbackDeploymentTransaction(tx); recoveryErr != nil {
				return 0, fmt.Errorf("candidate failed (%v); previous release recovery failed (%v)", startErr, recoveryErr)
			}
		}
		_ = m.removeDeploymentTransaction(tx.SiteID)
		return 0, startErr
	}

	candidate, ok := m.registry.Get(tx.SiteID)
	if !ok {
		if recoveryErr := m.rollbackDeploymentTransaction(tx); recoveryErr != nil {
			return 0, fmt.Errorf("candidate process record disappeared; recovery failed: %w", recoveryErr)
		}
		_ = m.removeDeploymentTransaction(tx.SiteID)
		return 0, fmt.Errorf("candidate process record disappeared")
	}
	candidate.Port = publicPort
	candidate.BackendPort = backendPort
	if err := m.registry.Save(candidate); err != nil {
		if recoveryErr := m.rollbackDeploymentTransaction(tx); recoveryErr != nil {
			return 0, fmt.Errorf("persist candidate backend identity failed (%v); recovery failed (%v)", err, recoveryErr)
		}
		_ = m.removeDeploymentTransaction(tx.SiteID)
		return 0, fmt.Errorf("persist candidate backend identity: %w", err)
	}
	m.mu.Lock()
	if process := m.procs[tx.SiteID]; process != nil && process.PID == pid { process.Port = publicPort }
	m.mu.Unlock()

	if healthPath != "" {
		if err := waitForHTTPContext(ctx, backendPort, healthPath, 15*time.Second); err != nil {
			if recoveryErr := m.rollbackDeploymentTransaction(tx); recoveryErr != nil {
				return 0, fmt.Errorf("candidate health check failed (%v); previous release recovery failed (%v)", err, recoveryErr)
			}
			_ = m.removeDeploymentTransaction(tx.SiteID)
			return 0, fmt.Errorf("candidate release failed health check: %w", err)
		}
	}

	// Ensure the stable listener is already serving the previous backend before
	// switching when the site has previously adopted blue-green mode.
	if tx.HadPrevious && tx.Previous.BackendPort > 0 {
		if err := m.ensureDeploymentProxy(tx.SiteID, publicPort, tx.Previous.BackendPort); err != nil {
			if recoveryErr := m.rollbackDeploymentTransaction(tx); recoveryErr != nil {
				return 0, fmt.Errorf("restore stable listener failed (%v); release recovery failed (%v)", err, recoveryErr)
			}
			_ = m.removeDeploymentTransaction(tx.SiteID)
			return 0, err
		}
		if err := m.switchDeploymentProxy(tx.SiteID, backendPort); err != nil {
			if recoveryErr := m.rollbackDeploymentTransaction(tx); recoveryErr != nil {
				return 0, fmt.Errorf("switch stable listener failed (%v); release recovery failed (%v)", err, recoveryErr)
			}
			_ = m.removeDeploymentTransaction(tx.SiteID)
			return 0, err
		}
		if err := m.stopDeploymentProcess(tx.Previous); err != nil {
			_ = m.switchDeploymentProxy(tx.SiteID, tx.Previous.BackendPort)
			if recoveryErr := m.rollbackDeploymentTransaction(tx); recoveryErr != nil {
				return 0, fmt.Errorf("old release shutdown failed (%v); recovery failed (%v)", err, recoveryErr)
			}
			_ = m.removeDeploymentTransaction(tx.SiteID)
			return 0, err
		}
	} else {
		// A legacy direct listener owns the public port. Stop it only after the
		// candidate is healthy, then bind the stable proxy to that port.
		if tx.HadPrevious {
			if err := m.stopDeploymentProcess(tx.Previous); err != nil {
				if recoveryErr := m.rollbackDeploymentTransaction(tx); recoveryErr != nil {
					return 0, fmt.Errorf("legacy release shutdown failed (%v); recovery failed (%v)", err, recoveryErr)
				}
				_ = m.removeDeploymentTransaction(tx.SiteID)
				return 0, err
			}
		}
		if err := m.ensureDeploymentProxy(tx.SiteID, publicPort, backendPort); err != nil {
			if recoveryErr := m.rollbackDeploymentTransaction(tx); recoveryErr != nil {
				return 0, fmt.Errorf("stable listener activation failed (%v); recovery failed (%v)", err, recoveryErr)
			}
			_ = m.removeDeploymentTransaction(tx.SiteID)
			return 0, err
		}
	}

	tx.State = "committed"
	if err := m.persistDeploymentTransaction(tx); err != nil {
		if recoveryErr := m.rollbackDeploymentTransaction(tx); recoveryErr != nil {
			return 0, fmt.Errorf("release became healthy but commit failed (%v); recovery failed (%v)", err, recoveryErr)
		}
		_ = m.removeDeploymentTransaction(tx.SiteID)
		return 0, fmt.Errorf("release commit failed; previous release restored: %w", err)
	}
	if err := m.removeDeploymentTransaction(tx.SiteID); err != nil {
		return 0, fmt.Errorf("release committed but recovery journal cleanup failed: %w", err)
	}
	return pid, nil
}
