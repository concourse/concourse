package metric

import (
	"time"

	"github.com/bigdatadev/goryman"
	"github.com/pivotal-golang/lager"
)

func periodicallyEmit(logger lager.Logger, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		tLog := logger.Session("tick")

		emit(eventEmission{
			logger: tLog,

			event: goryman.Event{
				Service: "tracked containers",
				Metric:  TrackedContainers.Max(),
				State:   "ok",
			},
		})
	}
}
