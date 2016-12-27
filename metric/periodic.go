package metric

import (
	"runtime"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/The-Cloud-Source/goryman"
)

func PeriodicallyEmit(logger lager.Logger, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		tLog := logger.Session("tick")

		trackedContainers := TrackedContainers.Max()
		trackedVolumes := TrackedVolumes.Max()
		databaseQueries := DatabaseQueries.Delta()
		databaseConnections := DatabaseConnections.Max()

		emit(
			tLog.Session("tracked-containers", lager.Data{
				"count": trackedContainers,
			}),
			goryman.Event{
				Service: "tracked containers",
				Metric:  trackedContainers,
				State:   "ok",
			},
		)

		emit(
			tLog.Session("tracked-volumes", lager.Data{
				"count": trackedVolumes,
			}),
			goryman.Event{
				Service: "tracked volumes",
				Metric:  trackedVolumes,
				State:   "ok",
			},
		)

		emit(
			tLog.Session("database-queries", lager.Data{
				"count": databaseQueries,
			}),
			goryman.Event{
				Service: "database queries",
				Metric:  databaseQueries,
				State:   "ok",
			},
		)

		emit(
			tLog.Session("database-connections", lager.Data{
				"count": databaseConnections,
			}),
			goryman.Event{
				Service: "database connections",
				Metric:  databaseConnections,
				State:   "ok",
			},
		)

		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		emit(
			tLog.Session("gc-pause-total-duration", lager.Data{
				"ns": memStats.PauseTotalNs,
			}),
			goryman.Event{
				Service: "gc pause total duration",
				Metric:  int(memStats.PauseTotalNs),
				State:   "ok",
			},
		)

		emit(
			tLog.Session("mallocs", lager.Data{
				"count": memStats.Mallocs,
			}),
			goryman.Event{
				Service: "mallocs",
				Metric:  int(memStats.Mallocs),
				State:   "ok",
			},
		)

		emit(
			tLog.Session("frees", lager.Data{
				"count": memStats.Frees,
			}),
			goryman.Event{
				Service: "frees",
				Metric:  int(memStats.Frees),
				State:   "ok",
			},
		)

		emit(
			tLog.Session("goroutines", lager.Data{
				"count": runtime.NumGoroutine(),
			}),
			goryman.Event{
				Service: "goroutines",
				Metric:  int(runtime.NumGoroutine()),
				State:   "ok",
			},
		)
	}
}
