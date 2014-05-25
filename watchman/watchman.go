package watchman

import (
	"sync"
	"time"

	"github.com/winston-ci/winston/builder"
	"github.com/winston-ci/winston/config"
	"github.com/winston-ci/winston/resources"
)

type Watchman interface {
	Watch(
		job config.Job,
		resource config.Resource,
		checker resources.Checker,
		interval time.Duration,
	)

	Stop()
}

type watchman struct {
	builder builder.Builder

	stop     chan struct{}
	watching *sync.WaitGroup
}

func NewWatchman(builder builder.Builder) Watchman {
	return &watchman{
		builder: builder,

		stop:     make(chan struct{}),
		watching: new(sync.WaitGroup),
	}
}

func (watchman *watchman) Watch(
	job config.Job,
	resource config.Resource,
	checker resources.Checker,
	interval time.Duration,
) {
	watchman.watching.Add(1)

	go func() {
		defer watchman.watching.Done()

		ticker := time.NewTicker(interval)

		for {
			select {
			case <-watchman.stop:
				return
			case <-ticker.C:
				for _, resource = range checker.CheckResource(resource) {
					watchman.builder.Build(job, resource)
				}
			}
		}
	}()
}

func (watchman *watchman) Stop() {
	close(watchman.stop)
	watchman.watching.Wait()
}
