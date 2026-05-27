package atc

import "time"

// Health represents the overall health status of the Concourse ATC instance
type Health struct {
	Status    string        `json:"status"`
	Version   string        `json:"version,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	Details   HealthDetails `json:"details"`
}

// HealthDetails contains detailed health information for different components
type HealthDetails struct {
	Database     HealthStatus `json:"database"`
	WorkerHealth HealthStatus `json:"worker_health"`
}

// HealthStatus represents the health status of a specific component
type HealthStatus struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// Health status constants
const (
	HealthStatusHealthy   = "healthy"
	HealthStatusUnhealthy = "unhealthy"
	HealthStatusDegraded  = "degraded"
)
