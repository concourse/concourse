package healthserver

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) GetHealth(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("get-health")

	health := atc.Health{
		Timestamp: time.Now().UTC(),
	}

	dbHealth := s.checkDatabase(r.Context())
	health.Database = dbHealth

	workerHealth := s.checkWorkers()
	health.Workers = workerHealth

	componentHealths, runtimeDegraded := s.checkComponents()
	health.Components = componentHealths

	switch {
	case dbHealth.Status == atc.HealthStatusUnhealthy:
		health.Status = atc.HealthStatusFailing
	case workerHealth.Status == atc.HealthStatusUnhealthy:
		health.Status = atc.HealthStatusFailing
	case workerHealth.Status == atc.HealthStatusDegraded || runtimeDegraded:
		health.Status = atc.HealthStatusDegraded
	default:
		health.Status = atc.HealthStatusOK
	}

	w.Header().Set("Content-Type", "application/json")
	if health.Status == atc.HealthStatusFailing {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	if err := json.NewEncoder(w).Encode(health); err != nil {
		logger.Error("failed-to-encode-health", err)
	}
}

// checkDatabase pings the database and verifies it is writable (not a standby replica).
func (s *Server) checkDatabase(ctx context.Context) atc.DatabaseHealth {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var inRecovery bool
	err := s.dbConn.QueryRowContext(ctx, "SELECT pg_is_in_recovery()").Scan(&inRecovery)
	if err != nil {
		return atc.DatabaseHealth{
			Status: atc.HealthStatusUnhealthy,
			Error:  "database unreachable",
		}
	}
	if inRecovery {
		return atc.DatabaseHealth{
			Status: atc.HealthStatusUnhealthy,
			Error:  "database is read-only",
		}
	}
	return atc.DatabaseHealth{Status: atc.HealthStatusHealthy}
}

// checkWorkers aggregates worker states against the configured minimum.
func (s *Server) checkWorkers() atc.WorkerHealth {
	workers, err := s.workerFactory.Workers()
	if err != nil {
		return atc.WorkerHealth{Status: atc.HealthStatusUnhealthy}
	}

	total := len(workers)
	running := countWorkers(workers, db.WorkerStateRunning)

	switch {
	case total == 0 || running == 0:
		return atc.WorkerHealth{Status: atc.HealthStatusUnhealthy, Total: total, Running: running}
	case running < s.minWorkerCount:
		return atc.WorkerHealth{Status: atc.HealthStatusDegraded, Total: total, Running: running}
	default:
		return atc.WorkerHealth{Status: atc.HealthStatusHealthy, Total: total, Running: running}
	}
}

// checkComponents fetches all components and checks their staleness.
// Returns the per-component health list and whether any runtime component is stale.
func (s *Server) checkComponents() ([]atc.ComponentHealth, bool) {
	components, err := s.componentFactory.All()
	if err != nil {
		return nil, false
	}

	runtimeSet := make(map[string]struct{}, len(atc.ComponentsRuntime))
	for _, name := range atc.ComponentsRuntime {
		runtimeSet[name] = struct{}{}
	}

	var healths []atc.ComponentHealth
	runtimeDegraded := false

	for _, c := range components {
		stale := isStale(c, s.componentStaleMultiplier)
		status := atc.HealthStatusHealthy
		if stale {
			status = atc.HealthStatusUnhealthy
			if _, isRuntime := runtimeSet[c.Name()]; isRuntime {
				runtimeDegraded = true
			}
		}

		healths = append(healths, atc.ComponentHealth{
			Name:    c.Name(),
			Status:  status,
			Paused:  c.Paused(),
			LastRan: c.LastRan(),
		})
	}

	return healths, runtimeDegraded
}

// isStale returns true if the component has not run within the stale window.
// Paused components and components that have never run are never considered stale.
func isStale(c db.Component, multiplier float64) bool {
	if c.Paused() || c.LastRan().IsZero() {
		return false
	}
	staleWindow := time.Duration(float64(c.Interval()) * multiplier)
	return time.Since(c.LastRan()) > staleWindow
}

func countWorkers(workers []db.Worker, state db.WorkerState) int {
	n := 0
	for _, w := range workers {
		if w.State() == state {
			n++
		}
	}
	return n
}
