package agent

import (
 "encoding/json"
 "fmt"
 "os"
 "path/filepath"
 "time"
)

// cloneEnv prevents caller-owned environment maps from mutating persisted launch config.
func cloneEnv(src map[string]string) map[string]string {
 if len(src)==0 { return nil }
 dst:=make(map[string]string,len(src)); for k,v:=range src { dst[k]=v }; return dst
}

const maxAutomaticRestarts = 3

// restartDelay is bounded exponential backoff: 2s, 4s, 8s.
func shouldRecoverAfterAgentRestart(policy string) bool {
 return policy == "always" || policy == "on-failure" || policy == ""
}

func shouldRestart(policy string, waitErr error) bool {
 if policy == "never" { return false }
 if policy == "always" { return true }
 return policy == "on-failure" && waitErr != nil
}

func restartDelay(attempt int) time.Duration {
 if attempt<1 { attempt=1 }; if attempt>3 { attempt=3 }
 return time.Duration(1<<attempt)*time.Second
}


// validRestartPolicy accepts the three supported supervisor policies.
func validRestartPolicy(policy string) bool {
 return policy == "never" || policy == "on-failure" || policy == "always"
}

func restartPoliciesPath(dataRoot string) string {
 return filepath.Join(dataRoot, "restart-policies.json")
}

func readRestartPolicies(dataRoot string) (map[string]string, error) {
 data, err := os.ReadFile(restartPoliciesPath(dataRoot))
 if os.IsNotExist(err) { return map[string]string{}, nil }
 if err != nil { return nil, err }
 policies := map[string]string{}
 if err := json.Unmarshal(data, &policies); err != nil { return nil, fmt.Errorf("decode restart policies: %w", err) }
 for site, policy := range policies {
  if !validSiteID(site) || !validRestartPolicy(policy) { return nil, fmt.Errorf("invalid persisted restart policy for site %q", site) }
 }
 return policies, nil
}

func writeRestartPolicies(dataRoot string, policies map[string]string) error {
 if err := os.MkdirAll(dataRoot, 0700); err != nil { return err }
 data, err := json.MarshalIndent(policies, "", "  "); if err != nil { return err }
 path := restartPoliciesPath(dataRoot)
 tmp := path + ".tmp"
 if err := os.WriteFile(tmp, data, 0600); err != nil { return err }
 if err := os.Rename(tmp, path); err != nil { _ = os.Remove(tmp); return err }
 return nil
}

// SetRestartPolicy persists a policy independently of process state, so it
// survives manual stops and Agent restarts.
func (m *ProcessManager) SetRestartPolicy(siteID, policy string) error {
 m.operationMu.Lock()
 defer m.operationMu.Unlock()
 if !validSiteID(siteID) { return fmt.Errorf("invalid site id") }
 if !validRestartPolicy(policy) { return fmt.Errorf("restart policy must be never, on-failure or always") }
 policies, err := readRestartPolicies(m.DataRoot); if err != nil { return err }
 policies[siteID] = policy
 if err := writeRestartPolicies(m.DataRoot, policies); err != nil { return fmt.Errorf("persist restart policy: %w", err) }
 if m.registry != nil {
  if record, ok := m.registry.Get(siteID); ok {
   record.RestartPolicy = policy
   if err := m.registry.Save(record); err != nil { return fmt.Errorf("update running process policy: %w", err) }
  }
 }
 return nil
}

func (m *ProcessManager) GetRestartPolicy(siteID string) (string, error) {
 if !validSiteID(siteID) { return "", fmt.Errorf("invalid site id") }
 policies, err := readRestartPolicies(m.DataRoot); if err != nil { return "", err }
 if policy := policies[siteID]; policy != "" { return policy, nil }
 return "on-failure", nil
}
