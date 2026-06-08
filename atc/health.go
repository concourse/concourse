package atc

import "time"

// Health represents the overall health status of the Concourse ATC instance.
type Health struct {
	Status    string         `json:"status"`
	Timestamp time.Time      `json:"timestamp"`
	Database  DatabaseHealth `json:"database"`
	Workers   WorkerHealth   `json:"workers"`
}

// DatabaseHealth represents the health of the database connection.
type DatabaseHealth struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// WorkerHealth represents the aggregate health of registered workers.
type WorkerHealth struct {
	Status  string `json:"status"`
	Total   int    `json:"total"`
	Running int    `json:"running"`
}

// Overall health status values — used in Health.Status.
const (
	HealthStatusOK       = "ok"
	HealthStatusDegraded = "degraded"
	HealthStatusFailing  = "failing"
)

// Per-subsystem status values — used in DatabaseHealth.Status and WorkerHealth.Status.
const (
	HealthStatusHealthy   = "healthy"
	HealthStatusUnhealthy = "unhealthy"
)
