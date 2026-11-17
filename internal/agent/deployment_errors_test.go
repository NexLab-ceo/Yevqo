package agent

import (
	"context"
	"errors"
	"testing"
)

func TestClassifyDeploymentFailure(t *testing.T) {
	cases := []struct {
		name string
		err error
		code string
		retryable bool
	}{
		{"cancelled", context.Canceled, "deployment_cancelled", false},
		{"deadline", context.DeadlineExceeded, "deployment_timeout", true},
		{"git timeout", errors.New("Git checkout timed out after 5m"), "git_checkout_timeout", true},
		{"build", errors.New("build failed: compiler exited with status 1"), "build_failed", false},
		{"health", errors.New("candidate release failed health check: status 503"), "health_check_failed", true},
		{"invalid config", errors.New("invalid Git branch"), "invalid_deployment_configuration", false},
		{"fallback", errors.New("could not persist deployment journal"), "deployment_failed", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyDeploymentFailure(tc.err)
			if got.Code != tc.code || got.Retryable != tc.retryable {
				t.Fatalf("classification = %#v, want code=%q retryable=%v", got, tc.code, tc.retryable)
			}
		})
	}
}
