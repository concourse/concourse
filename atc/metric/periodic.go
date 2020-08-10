package metric

import (
	"os"
	"runtime"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"
)

func PeriodicallyEmit(logger lager.Logger, interval time.Duration) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		close(ready)

		for {
			select {
			case <-signals:
				return nil
			case <-ticker.C:
				tick(logger.Session("tick"))
			}
		}
	})
}

func tick(logger lager.Logger) {
	Metrics.emit(
		logger.Session("database-queries"),
		Event{
			Name:  "database queries",
			Value: DatabaseQueries.Delta(),
		},
	)

	if len(Databases) > 0 {
		for _, database := range Databases {
			Metrics.emit(
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

	Metrics.emit(
		logger.Session("containers-deleted"),
		Event{
			Name:  "containers deleted",
			Value: ContainersDeleted.Delta(),
		},
	)

	Metrics.emit(
		logger.Session("volumes-deleted"),
		Event{
			Name:  "volumes deleted",
			Value: VolumesDeleted.Delta(),
		},
	)

	Metrics.emit(
		logger.Session("checks-deleted"),
		Event{
			Name:  "checks deleted",
			Value: ChecksDeleted.Delta(),
		},
	)

	Metrics.emit(
		logger.Session("containers-created"),
		Event{
			Name:  "containers created",
			Value: ContainersCreated.Delta(),
		},
	)

	Metrics.emit(
		logger.Session("volumes-created"),
		Event{
			Name:  "volumes created",
			Value: VolumesCreated.Delta(),
		},
	)

	Metrics.emit(
		logger.Session("failed-containers"),
		Event{
			Name:  "failed containers",
			Value: FailedContainers.Delta(),
		},
	)

	Metrics.emit(
		logger.Session("failed-volumes"),
		Event{
			Name:  "failed volumes",
			Value: FailedVolumes.Delta(),
		},
	)

	Metrics.emit(
		logger.Session("jobs-scheduled"),
		Event{
			Name:  "jobs scheduled",
			Value: JobsScheduled.Delta(),
		},
	)

	Metrics.emit(
		logger.Session("jobs-scheduling"),
		Event{
			Name:  "jobs scheduling",
			Value: JobsScheduling.Max(),
		},
	)

	Metrics.emit(
		logger.Session("builds-started"),
		Event{
			Name:  "builds started",
			Value: BuildsStarted.Delta(),
		},
	)

	Metrics.emit(
		logger.Session("builds-running"),
		Event{
			Name:  "builds running",
			Value: BuildsRunning.Max(),
		},
	)

	for action, gauge := range ConcurrentRequests {
		Metrics.emit(
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

	for action, counter := range ConcurrentRequestsLimitHit {
		Metrics.emit(
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

	Metrics.emit(
		logger.Session("tasks-waiting"),
		Event{
			Name:  "tasks waiting",
			Value: TasksWaiting.Max(),
		},
	)

	Metrics.emit(
		logger.Session("checks-finished-with-error"),
		Event{
			Name:  "checks finished",
			Value: ChecksFinishedWithError.Delta(),
			Attributes: map[string]string{
				"status": "error",
			},
		},
	)

	Metrics.emit(
		logger.Session("checks-finished-with-success"),
		Event{
			Name:  "checks finished",
			Value: ChecksFinishedWithSuccess.Delta(),
			Attributes: map[string]string{
				"status": "success",
			},
		},
	)

	Metrics.emit(

		logger.Session("checks-started"),
		Event{
			Name:  "checks started",
			Value: ChecksStarted.Delta(),
		},
	)

	Metrics.emit(

		logger.Session("checks-enqueued"),
		Event{
			Name:  "checks enqueued",
			Value: ChecksEnqueued.Delta(),
		},
	)

	Metrics.emit(
		logger.Session("checks-queue-size"),
		Event{
			Name:  "checks queue size",
			Value: ChecksQueueSize.Max(),
		},
	)

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	Metrics.emit(
		logger.Session("gc-pause-total-duration"),
		Event{
			Name:  "gc pause total duration",
			Value: float64(memStats.PauseTotalNs),
		},
	)

	Metrics.emit(
		logger.Session("mallocs"),
		Event{
			Name:  "mallocs",
			Value: float64(memStats.Mallocs),
		},
	)

	Metrics.emit(
		logger.Session("frees"),
		Event{
			Name:  "frees",
			Value: float64(memStats.Frees),
		},
	)

	Metrics.emit(
		logger.Session("goroutines"),
		Event{
			Name:  "goroutines",
			Value: float64(runtime.NumGoroutine()),
		},
	)
}
