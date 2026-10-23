package healthserver

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/concourse/concourse/v8/atc"
	"github.com/concourse/concourse/v8/atc/db"
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
	case workerHealth.Status == string(atc.HealthStatusUnhealthy):
		health.Status = atc.HealthStatusFailing
	case workerHealth.Status == string(atc.HealthStatusDegraded) || runtimeDegraded:
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
		http.Error(w, "failed to encode health response", http.StatusInternalServerError)
	}
}

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

func (s *Server) checkWorkers() atc.WorkerHealth {
	workers, err := s.workerFactory.Workers()
	if err != nil {
		return atc.WorkerHealth{Status: string(atc.HealthStatusUnhealthy)}
	}

	total := len(workers)
	running := 0
	var unhealthy []string

	byPlatform := make(map[string]atc.WorkerBreakdown)
	byTeam := make(map[string]atc.WorkerBreakdown)
	byTag := make(map[string]atc.WorkerBreakdown)

	for _, w := range workers {
		isRunning := w.State() == db.WorkerStateRunning
		if isRunning {
			running++
		} else {
			unhealthy = append(unhealthy, w.Name())
		}

		platform := w.Platform()
		bd := byPlatform[platform]
		bd.Total++
		if isRunning {
			bd.Running++
		}
		byPlatform[platform] = bd

		team := w.TeamName()
		bt := byTeam[team]
		bt.Total++
		if isRunning {
			bt.Running++
		}
		byTeam[team] = bt

		for _, tag := range w.Tags() {
			btag := byTag[tag]
			btag.Total++
			if isRunning {
				btag.Running++
			}
			byTag[tag] = btag
		}
	}

	// Inject zero entries so configured-but-absent pools are visible without auth.
	for name := range s.minWorkerThresholds.Platform {
		if _, ok := byPlatform[name]; !ok {
			byPlatform[name] = atc.WorkerBreakdown{}
		}
	}
	for name := range s.minWorkerThresholds.Team {
		if _, ok := byTeam[name]; !ok {
			byTeam[name] = atc.WorkerBreakdown{}
		}
	}
	for name := range s.minWorkerThresholds.Tag {
		if _, ok := byTag[name]; !ok {
			byTag[name] = atc.WorkerBreakdown{}
		}
	}

	pfFailing, pfDegraded := thresholdStatus(byPlatform, s.minWorkerThresholds.Platform)
	tmFailing, tmDegraded := thresholdStatus(byTeam, s.minWorkerThresholds.Team)
	tgFailing, tgDegraded := thresholdStatus(byTag, s.minWorkerThresholds.Tag)

	switch {
	case total == 0 || running == 0 || pfFailing || tmFailing || tgFailing:
		return atc.WorkerHealth{
			Status:           string(atc.HealthStatusUnhealthy),
			Total:            total,
			Running:          running,
			ByPlatform:       byPlatform,
			ByTeam:           byTeam,
			ByTag:            byTag,
			UnhealthyWorkers: unhealthy,
		}
	case running < s.minWorkerCount || pfDegraded || tmDegraded || tgDegraded:
		return atc.WorkerHealth{
			Status:           string(atc.HealthStatusDegraded),
			Total:            total,
			Running:          running,
			ByPlatform:       byPlatform,
			ByTeam:           byTeam,
			ByTag:            byTag,
			UnhealthyWorkers: unhealthy,
		}
	default:
		return atc.WorkerHealth{
			Status:           string(atc.HealthStatusHealthy),
			Total:            total,
			Running:          running,
			ByPlatform:       byPlatform,
			ByTeam:           byTeam,
			ByTag:            byTag,
			UnhealthyWorkers: unhealthy,
		}
	}
}

func thresholdStatus(counts map[string]atc.WorkerBreakdown, entries map[string]ThresholdEntry) (failing, degraded bool) {
	for name, entry := range entries {
		if entry.Severity == ThresholdSeverityInfo {
			continue
		}
		running := counts[name].Running
		if running >= entry.Min {
			continue
		}
		if entry.Severity == ThresholdSeverityCritical && running == 0 {
			failing = true
		} else {
			degraded = true
		}
	}
	return
}

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

func isStale(c db.Component, multiplier float64) bool {
	if c.Paused() || c.LastRan().IsZero() {
		return false
	}
	staleWindow := time.Duration(float64(c.Interval()) * multiplier)
	return time.Since(c.LastRan()) > staleWindow
}
