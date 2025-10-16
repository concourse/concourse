package metric

import (
	"os"
	"runtime"
	"time"

	"code.cloudfoundry.org/lager/v3"
	"github.com/tedsuo/ifrit"
)

func PeriodicallyEmit(logger lager.Logger, m *Monitor, interval time.Duration) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		close(ready)

		for {
			select {
			case <-signals:
				return nil
			case <-ticker.C:
				tick(logger.Session("tick"), m)
			}
		}
	})
}

func tick(logger lager.Logger, m *Monitor) {
	m.emit(
		logger.Session("database-queries"),
		Event{
			Name:  "database queries",
			Value: m.DatabaseQueries.Delta(),
		},
	)

	if len(m.Databases) > 0 {
		for _, database := range m.Databases {
			m.emit(
				logger.Session("database-connections"),
				Event{
					Name:  "database connections",
					Value: float64(database.Stats().OpenConnections),
					Attributes: map[string]string{
						"ConnectionName": database.Name(),
					},
				},
			)
		}
	}

	m.emit(
		logger.Session("containers-deleted"),
		Event{
			Name:  "containers deleted",
			Value: m.ContainersDeleted.Delta(),
		},
	)

	m.emit(
		logger.Session("volumes-deleted"),
		Event{
			Name:  "volumes deleted",
			Value: m.VolumesDeleted.Delta(),
		},
	)

	m.emit(
		logger.Session("volumes-streamed"),
		Event{
			Name:  "volumes streamed",
			Value: m.VolumesStreamed.Delta(),
		},
	)

	m.emit(
		logger.Session("get-step-cache-hits"),
		Event{
			Name:  "get step cache hits",
			Value: m.GetStepCacheHits.Delta(),
		},
	)

	m.emit(
		logger.Session("streamed-resource-caches"),
		Event{
			Name:  "streamed resource caches",
			Value: m.StreamedResourceCaches.Delta(),
		},
	)

	m.emit(
		logger.Session("containers-created"),
		Event{
			Name:  "containers created",
			Value: m.ContainersCreated.Delta(),
		},
	)

	m.emit(
		logger.Session("volumes-created"),
		Event{
			Name:  "volumes created",
			Value: m.VolumesCreated.Delta(),
		},
	)

	m.emit(
		logger.Session("failed-containers"),
		Event{
			Name:  "failed containers",
			Value: m.FailedContainers.Delta(),
		},
	)

	m.emit(
		logger.Session("failed-volumes"),
		Event{
			Name:  "failed volumes",
			Value: m.FailedVolumes.Delta(),
		},
	)

	m.emit(
		logger.Session("jobs-scheduled"),
		Event{
			Name:  "jobs scheduled",
			Value: m.JobsScheduled.Delta(),
		},
	)

	m.emit(
		logger.Session("jobs-scheduling"),
		Event{
			Name:  "jobs scheduling",
			Value: m.JobsScheduling.Max(),
		},
	)

	m.emit(
		logger.Session("builds-started"),
		Event{
			Name:  "builds started",
			Value: m.BuildsStarted.Delta(),
		},
	)

	m.emit(
		logger.Session("builds-running"),
		Event{
			Name:  "builds running",
			Value: m.BuildsRunning.Max(),
		},
	)

	m.emit(
		logger.Session("check-builds-started"),
		Event{
			Name:  "check builds started",
			Value: m.CheckBuildsStarted.Delta(),
		},
	)

	m.emit(
		logger.Session("check-builds-running"),
		Event{
			Name:  "check builds running",
			Value: m.CheckBuildsRunning.Max(),
		},
	)

	for action, gauge := range m.ConcurrentRequests {
		m.emit(
			logger.Session("concurrent-requests"),
			Event{
				Name:  "concurrent requests",
				Value: gauge.Max(),
				Attributes: map[string]string{
					"action": action,
				},
			},
		)
	}

	for action, counter := range m.ConcurrentRequestsLimitHit {
		m.emit(
			logger.Session("concurrent-requests-limit-hit"),
			Event{
				Name:  "concurrent requests limit hit",
				Value: counter.Delta(),
				Attributes: map[string]string{
					"action": action,
				},
			},
		)
	}

	for labels, gauge := range m.StepsWaiting {
		m.emit(
			logger.Session("steps-waiting"),
			Event{
				Name:  "steps waiting",
				Value: gauge.Max(),
				Attributes: map[string]string{
					"platform":   labels.Platform,
					"teamId":     labels.TeamId,
					"teamName":   labels.TeamName,
					"type":       labels.Type,
					"workerTags": labels.WorkerTags,
				},
			},
		)
	}

	m.emit(
		logger.Session("checks-finished-with-error"),
		Event{
			Name:  "checks finished",
			Value: m.ChecksFinishedWithError.Delta(),
			Attributes: map[string]string{
				"status": "error",
			},
		},
	)

	m.emit(
		logger.Session("checks-finished-with-success"),
		Event{
			Name:  "checks finished",
			Value: m.ChecksFinishedWithSuccess.Delta(),
			Attributes: map[string]string{
				"status": "success",
			},
		},
	)

	m.emit(
		logger.Session("checks-started"),
		Event{
			Name:  "checks started",
			Value: m.ChecksStarted.Delta(),
		},
	)

	m.emit(
		logger.Session("checks-enqueued"),
		Event{
			Name:  "checks enqueued",
			Value: m.ChecksEnqueued.Delta(),
		},
	)

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	m.emit(
		logger.Session("gc-pause-total-duration"),
		Event{
			Name:  "gc pause total duration",
			Value: float64(memStats.PauseTotalNs),
		},
	)

	m.emit(
		logger.Session("mallocs"),
		Event{
			Name:  "mallocs",
			Value: float64(memStats.Mallocs),
		},
	)

	m.emit(
		logger.Session("frees"),
		Event{
			Name:  "frees",
			Value: float64(memStats.Frees),
		},
	)

	m.emit(
		logger.Session("heap-alloc"),
		Event{
			Name:  "heap alloc",
			Value: float64(memStats.HeapAlloc),
		},
	)

	m.emit(
		logger.Session("heap-inuse"),
		Event{
			Name:  "heap inuse",
			Value: float64(memStats.HeapInuse),
		},
	)

	m.emit(
		logger.Session("heap-objects"),
		Event{
			Name:  "heap objects",
			Value: float64(memStats.HeapObjects),
		},
	)

	m.emit(
		logger.Session("stack-inuse"),
		Event{
			Name:  "stack inuse",
			Value: float64(memStats.StackInuse),
		},
	)

	m.emit(
		logger.Session("sys"),
		Event{
			Name:  "sys",
			Value: float64(memStats.Sys),
		},
	)

	m.emit(
		logger.Session("num-gc"),
		Event{
			Name:  "num gc",
			Value: float64(memStats.NumGC),
		},
	)

	m.emit(
		logger.Session("gc-cpu-fraction"),
		Event{
			Name:  "gc cpu fraction",
			Value: memStats.GCCPUFraction * 100,
		},
	)

	m.emit(
		logger.Session("next-gc"),
		Event{
			Name:  "next gc",
			Value: float64(memStats.NextGC),
		},
	)

	m.emit(
		logger.Session("num-cpu"),
		Event{
			Name:  "num cpu",
			Value: float64(runtime.NumCPU()),
		},
	)

	m.emit(
		logger.Session("goroutines"),
		Event{
			Name:  "goroutines",
			Value: float64(runtime.NumGoroutine()),
		},
	)
}
