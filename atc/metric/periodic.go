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
	emit(
		logger.Session("database-queries"),
		Event{
			Name:  "database queries",
			Value: DatabaseQueries.Delta(),
			State: EventStateOK,
		},
	)

	if len(Databases) > 0 {
		for _, database := range Databases {
			emit(
				logger.Session("database-connections"),
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
		logger.Session("checks-deleted"),
		Event{
			Name:  "checks deleted",
			Value: ChecksDeleted.Delta(),
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
		logger.Session("gc-pause-total-duration"),
		Event{
			Name:  "gc pause total duration",
			Value: int(memStats.PauseTotalNs),
			State: EventStateOK,
		},
	)

	emit(
		logger.Session("mallocs"),
		Event{
			Name:  "mallocs",
			Value: int(memStats.Mallocs),
			State: EventStateOK,
		},
	)

	emit(
		logger.Session("frees"),
		Event{
			Name:  "frees",
			Value: int(memStats.Frees),
			State: EventStateOK,
		},
	)

	emit(
		logger.Session("goroutines"),
		Event{
			Name:  "goroutines",
			Value: int(runtime.NumGoroutine()),
			State: EventStateOK,
		},
	)
}
