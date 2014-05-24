package watchman

import (
	"time"

	"github.com/winston-ci/winston/builder"
	"github.com/winston-ci/winston/config"
	"github.com/winston-ci/winston/resources"
)

type Watchman interface {
	Watch(
		job config.Job,
		resource config.Resource,
		resources config.Resources,
		checker resources.Checker,
		interval time.Duration,
	) (stop chan<- struct{})
}

type watchman struct {
	builder builder.Builder
}

func NewWatchman(builder builder.Builder) Watchman {
	return &watchman{
		builder: builder,
	}
}

func (watchman *watchman) Watch(
	job config.Job,
	resource config.Resource,
	resources config.Resources,
	checker resources.Checker,
	interval time.Duration,
) chan<- struct{} {
	stop := make(chan struct{})

	go func() {
		ticker := time.NewTicker(interval)

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				for _, resource = range checker.CheckResource(resource) {
					watchman.builder.Build(job, resources.UpdateResource(resource))
				}
			}
		}
	}()

	return stop
}
