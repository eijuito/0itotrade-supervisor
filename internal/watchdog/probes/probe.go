package probes

import (
	"context"
	"time"
)

type Status string

const (
	StatusHealthy  Status = "HEALTHY"
	StatusDegraded Status = "DEGRADED"
	StatusCritical Status = "CRITICAL"
)

type ProbeResult struct {
	Name         string                 `json:"name"`
	Target       string                 `json:"target"`
	Status       Status                 `json:"status"`
	LatencyMs    int64                  `json:"latency_ms"`
	LastSuccess  time.Time              `json:"last_success,omitempty"`
	Failures     int                    `json:"consecutive_failures"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
}

type Probe interface {
	Name() string
	Check(ctx context.Context) ProbeResult
	Interval() time.Duration
}
