package atc

import "time"

// systemStatus represents the overall health of the ATC instance.
// Valid values: ok, degraded, failing.
type systemStatus string

const (
	HealthStatusOK       systemStatus = "ok"
	HealthStatusDegraded systemStatus = "degraded"
	HealthStatusFailing  systemStatus = "failing"
)

// subsystemStatus represents the binary health of an individual subsystem (database, component).
// Valid values: healthy, unhealthy.
type subsystemStatus string

const (
	HealthStatusHealthy   subsystemStatus = "healthy"
	HealthStatusUnhealthy subsystemStatus = "unhealthy"
)

// Health represents the overall health status of the Concourse ATC instance.
type Health struct {
	Status     systemStatus      `json:"status"`
	Timestamp  time.Time         `json:"timestamp"`
	Database   DatabaseHealth    `json:"database"`
	Workers    WorkerHealth      `json:"workers"`
	Components []ComponentHealth `json:"components"`
}

// DatabaseHealth represents the health of the database connection.
type DatabaseHealth struct {
	Status subsystemStatus `json:"status"`
	Error  string          `json:"error,omitempty"`
}

// WorkerHealth represents the aggregate health of registered workers.
// Status is one of: "healthy", "degraded", "unhealthy" — workers uniquely span
// both binary and graduated health levels, so the field is typed as string.
type WorkerHealth struct {
	Status           string   `json:"status"`
	Total            int      `json:"total"`
	Running          int      `json:"running"`
	UnhealthyWorkers []string `json:"unhealthy_workers,omitempty"`
}

// ComponentHealth represents the health of a single ATC component.
type ComponentHealth struct {
	Name    string          `json:"name"`
	Status  subsystemStatus `json:"status"`
	Paused  bool            `json:"paused"`
	LastRan time.Time       `json:"last_ran"`
}
