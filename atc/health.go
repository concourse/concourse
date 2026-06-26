package atc

import "time"

// Health represents the overall health status of the Concourse ATC instance.
type Health struct {
	Status     string            `json:"status"`
	Timestamp  time.Time         `json:"timestamp"`
	Database   DatabaseHealth    `json:"database"`
	Workers    WorkerHealth      `json:"workers"`
	Components []ComponentHealth `json:"components"`
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

// ComponentHealth represents the health of a single ATC component.
type ComponentHealth struct {
	Name    string    `json:"name"`
	Status  string    `json:"status"`
	Paused  bool      `json:"paused"`
	LastRan time.Time `json:"last_ran"`
}

// Overall health status values — used in Health.Status.
const (
	HealthStatusOK       = "ok"
	HealthStatusDegraded = "degraded"
	HealthStatusFailing  = "failing"
)

// Per-subsystem status values — used in DatabaseHealth.Status, WorkerHealth.Status, ComponentHealth.Status.
const (
	HealthStatusHealthy   = "healthy"
	HealthStatusUnhealthy = "unhealthy"
)
