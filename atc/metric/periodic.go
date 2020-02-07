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
		},
	)

	if len(Databases) > 0 {
		for _, database := range Databases {
			emit(
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

	emit(
		logger.Session("containers-deleted"),
		Event{
			Name:  "containers deleted",
			Value: ContainersDeleted.Delta(),
		},
	)

	emit(
		logger.Session("volumes-deleted"),
		Event{
			Name:  "volumes deleted",
			Value: VolumesDeleted.Delta(),
		},
	)

	emit(
		logger.Session("checks-deleted"),
		Event{
			Name:  "checks deleted",
			Value: ChecksDeleted.Delta(),
		},
	)

	emit(
		logger.Session("containers-created"),
		Event{
			Name:  "containers created",
			Value: ContainersCreated.Delta(),
		},
	)

	emit(
		logger.Session("volumes-created"),
		Event{
			Name:  "volumes created",
			Value: VolumesCreated.Delta(),
		},
	)

	emit(
		logger.Session("failed-containers"),
		Event{
			Name:  "failed containers",
			Value: FailedContainers.Delta(),
		},
	)

	emit(
		logger.Session("failed-volumes"),
		Event{
			Name:  "failed volumes",
			Value: FailedVolumes.Delta(),
		},
	)

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	emit(
		logger.Session("gc-pause-total-duration"),
		Event{
			Name:  "gc pause total duration",
			Value: float64(memStats.PauseTotalNs),
		},
	)

	emit(
		logger.Session("mallocs"),
		Event{
			Name:  "mallocs",
			Value: float64(memStats.Mallocs),
		},
	)

	emit(
		logger.Session("frees"),
		Event{
			Name:  "frees",
			Value: float64(memStats.Frees),
		},
	)

	emit(
		logger.Session("goroutines"),
		Event{
			Name:  "goroutines",
			Value: float64(runtime.NumGoroutine()),
		},
	)
}
