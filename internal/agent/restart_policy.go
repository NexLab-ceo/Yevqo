package agent

import (
 "time"
)

// cloneEnv prevents caller-owned environment maps from mutating persisted launch config.
func cloneEnv(src map[string]string) map[string]string {
 if len(src)==0 { return nil }
 dst:=make(map[string]string,len(src)); for k,v:=range src { dst[k]=v }; return dst
}

const maxAutomaticRestarts = 3

// restartDelay is bounded exponential backoff: 2s, 4s, 8s.
func restartDelay(attempt int) time.Duration {
 if attempt<1 { attempt=1 }; if attempt>3 { attempt=3 }
 return time.Duration(1<<attempt)*time.Second
}
