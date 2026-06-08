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

	// --- Database health ---
	dbHealth := s.checkDatabase(r.Context())
	health.Database = dbHealth

	// --- Worker health ---
	workerHealth := s.checkWorkers()
	health.Workers = workerHealth

	// --- Overall status ---
	switch {
	case dbHealth.Status == atc.HealthStatusUnhealthy:
		health.Status = atc.HealthStatusFailing
	case workerHealth.Status == atc.HealthStatusUnhealthy:
		health.Status = atc.HealthStatusFailing
	case workerHealth.Status == atc.HealthStatusDegraded:
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
		return atc.WorkerHealth{
			Status: atc.HealthStatusUnhealthy,
		}
	}

	total := len(workers)
	running := countWorkers(workers, db.WorkerStateRunning)

	switch {
	case total == 0 || running == 0:
		return atc.WorkerHealth{
			Status:  atc.HealthStatusUnhealthy,
			Total:   total,
			Running: running,
		}
	case running < s.minWorkerCount:
		return atc.WorkerHealth{
			Status:  atc.HealthStatusDegraded,
			Total:   total,
			Running: running,
		}
	default:
		return atc.WorkerHealth{
			Status:  atc.HealthStatusHealthy,
			Total:   total,
			Running: running,
		}
	}
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
