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

		databaseQueries := DatabaseQueries.Delta()
		databaseConnections := DatabaseConnections.Max()

		emit(
			tLog.Session("database-queries", lager.Data{
				"count": databaseQueries,
			}),
			Event{
				Name:  "database queries",
				Value: databaseQueries,
				State: EventStateOK,
			},
		)

		emit(
			tLog.Session("database-connections", lager.Data{
				"count": databaseConnections,
			}),
			Event{
				Name:  "database connections",
				Value: databaseConnections,
				State: EventStateOK,
			},
		)

		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		emit(
			tLog.Session("gc-pause-total-duration", lager.Data{
				"ns": memStats.PauseTotalNs,
			}),
			Event{
				Name:  "gc pause total duration",
				Value: int(memStats.PauseTotalNs),
				State: EventStateOK,
			},
		)

		emit(
			tLog.Session("mallocs", lager.Data{
				"count": memStats.Mallocs,
			}),
			Event{
				Name:  "mallocs",
				Value: int(memStats.Mallocs),
				State: EventStateOK,
			},
		)

		emit(
			tLog.Session("frees", lager.Data{
				"count": memStats.Frees,
			}),
			Event{
				Name:  "frees",
				Value: int(memStats.Frees),
				State: EventStateOK,
			},
		)

		emit(
			tLog.Session("goroutines", lager.Data{
				"count": runtime.NumGoroutine(),
			}),
			Event{
				Name:  "goroutines",
				Value: int(runtime.NumGoroutine()),
				State: EventStateOK,
			},
		)
	}
}
