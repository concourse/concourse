package metric

import (
	"runtime"
	"time"

	"code.cloudfoundry.org/lager"
)

func PeriodicallyEmit(logger lager.Logger, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		tLog := logger.Session("tick")

		emit(
			tLog.Session("database-queries"),
			Event{
				Name:  "database queries",
				Value: DatabaseQueries.Delta(),
				State: EventStateOK,
			},
		)

		if len(Databases) > 0 {
			for _, database := range Databases {
				emit(
					tLog.Session("database-connections"),
					Event{
						Name:  "database connections",
						Value: database.Stats().OpenConnections,
						State: EventStateOK,
						Attributes: map[string]string{
							"ConnectionName": database.Name(),
						},
					},
				)
			}
		}

		emit(
			logger.Session("containers-deleted"),
			Event{
				Name:  "containers deleted",
				Value: ContainersDeleted.Delta(),
				State: EventStateOK,
			},
		)

		emit(
			logger.Session("volumes-deleted"),
			Event{
				Name:  "volumes deleted",
				Value: VolumesDeleted.Delta(),
				State: EventStateOK,
			},
		)

		emit(
			logger.Session("containers-created"),
			Event{
				Name:  "containers created",
				Value: ContainersCreated.Delta(),
				State: EventStateOK,
			},
		)

		emit(
			logger.Session("volumes-created"),
			Event{
				Name:  "volumes created",
				Value: VolumesCreated.Delta(),
				State: EventStateOK,
			},
		)

		emit(
			logger.Session("failed-containers"),
			Event{
				Name:  "failed containers",
				Value: FailedContainers.Delta(),
				State: EventStateOK,
			},
		)

		emit(
			logger.Session("failed-volumes"),
			Event{
				Name:  "failed volumes",
				Value: FailedVolumes.Delta(),
				State: EventStateOK,
			},
		)

		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		emit(
			tLog.Session("gc-pause-total-duration"),
			Event{
				Name:  "gc pause total duration",
				Value: int(memStats.PauseTotalNs),
				State: EventStateOK,
			},
		)

		emit(
			tLog.Session("mallocs"),
			Event{
				Name:  "mallocs",
				Value: int(memStats.Mallocs),
				State: EventStateOK,
			},
		)

		emit(
			tLog.Session("frees"),
			Event{
				Name:  "frees",
				Value: int(memStats.Frees),
				State: EventStateOK,
			},
		)

		emit(
			tLog.Session("goroutines"),
			Event{
				Name:  "goroutines",
				Value: int(runtime.NumGoroutine()),
				State: EventStateOK,
			},
		)
	}
}
