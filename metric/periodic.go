package metric

import (
	"time"

	"github.com/bigdatadev/goryman"
)

func periodicallyEmit(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		emit(eventEmission{
			event: goryman.Event{
				Service: "tracked containers",
				Metric:  TrackedContainers.Max(),
			},
		})
	}
}
