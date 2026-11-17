package agent

import (
	"context"
	"errors"
	"net"
	"strings"
)

type deploymentFailure struct {
	Code      string
	Retryable bool
}

func classifyDeploymentFailure(err error) deploymentFailure {
	if err == nil { return deploymentFailure{Code:"unknown_error"} }
	if errors.Is(err, context.Canceled) {
		return deploymentFailure{Code:"deployment_cancelled"}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return deploymentFailure{Code:"deployment_timeout", Retryable:true}
	}
	var networkErr net.Error
	if errors.As(err, &networkErr) && networkErr.Timeout() {
		return deploymentFailure{Code:"network_timeout", Retryable:true}
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "git checkout timed out"):
		return deploymentFailure{Code:"git_checkout_timeout", Retryable:true}
	case strings.Contains(message, "git checkout failed"), strings.Contains(message, "fetch requested commit"):
		return deploymentFailure{Code:"git_checkout_failed", Retryable:true}
	case strings.Contains(message, "health check"):
		return deploymentFailure{Code:"health_check_failed", Retryable:true}
	case strings.Contains(message, "readiness"):
		return deploymentFailure{Code:"readiness_failed", Retryable:true}
	case strings.Contains(message, "build failed"):
		return deploymentFailure{Code:"build_failed"}
	case strings.Contains(message, "release ") && strings.Contains(message, "already exists"):
		return deploymentFailure{Code:"release_conflict"}
	case strings.Contains(message, "invalid "), strings.Contains(message, "unsupported "), strings.Contains(message, "blocked:"), strings.Contains(message, "must be "):
		return deploymentFailure{Code:"invalid_deployment_configuration"}
	default:
		return deploymentFailure{Code:"deployment_failed"}
	}
}
