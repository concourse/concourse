package metric

import (
	"runtime"
	"time"

	"github.com/bigdatadev/goryman"
	"github.com/pivotal-golang/lager"
)

func periodicallyEmit(logger lager.Logger, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		tLog := logger.Session("tick")

		emit(tLog, goryman.Event{
			Service: "tracked containers",
			Metric:  TrackedContainers.Max(),
			State:   "ok",
		})

		emit(tLog, goryman.Event{
			Service: "tracked volumes",
			Metric:  TrackedVolumes.Max(),
			State:   "ok",
		})

		emit(tLog, goryman.Event{
			Service: "database queries",
			Metric:  DatabaseQueries.Delta(),
			State:   "ok",
		})

		emit(tLog, goryman.Event{
			Service: "database connections",
			Metric:  DatabaseConnections.Max(),
			State:   "ok",
		})

		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		emit(tLog, goryman.Event{
			Service: "gc pause total duration",
			Metric:  int(memStats.PauseTotalNs),
			State:   "ok",
		})

		emit(tLog, goryman.Event{
			Service: "mallocs",
			Metric:  int(memStats.Mallocs),
			State:   "ok",
		})

		emit(tLog, goryman.Event{
			Service: "frees",
			Metric:  int(memStats.Frees),
			State:   "ok",
		})

		emit(tLog, goryman.Event{
			Service: "goroutines",
			Metric:  int(runtime.NumGoroutine()),
			State:   "ok",
		})
	}
}
