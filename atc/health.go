package atc

import "time"

type systemStatus string

const (
	HealthStatusOK       systemStatus = "ok"
	HealthStatusDegraded systemStatus = "degraded"
	HealthStatusFailing  systemStatus = "failing"
)

type subsystemStatus string

const (
	HealthStatusHealthy   subsystemStatus = "healthy"
	HealthStatusUnhealthy subsystemStatus = "unhealthy"
)

type Health struct {
	Status     systemStatus      `json:"status"`
	Timestamp  time.Time         `json:"timestamp"`
	Database   DatabaseHealth    `json:"database"`
	Workers    WorkerHealth      `json:"workers"`
	Components []ComponentHealth `json:"components"`
}

type DatabaseHealth struct {
	Status subsystemStatus `json:"status"`
	Error  string          `json:"error,omitempty"`
}

type WorkerBreakdown struct {
	Total   int `json:"total"`
	Running int `json:"running"`
}

type WorkerHealth struct {
	Status           string                     `json:"status"`
	Total            int                        `json:"total"`
	Running          int                        `json:"running"`
	ByPlatform       map[string]WorkerBreakdown `json:"by_platform"`
	ByTeam           map[string]WorkerBreakdown `json:"by_team"`
	ByTag            map[string]WorkerBreakdown `json:"by_tag"`
	UnhealthyWorkers []string                   `json:"unhealthy_workers,omitempty"`
}

type ComponentHealth struct {
	Name    string          `json:"name"`
	Status  subsystemStatus `json:"status"`
	Paused  bool            `json:"paused"`
	LastRan time.Time       `json:"last_ran"`
}
