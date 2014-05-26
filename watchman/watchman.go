package watchman

import (
	"log"
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
		latestOnly bool,
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
	latestOnly bool,
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
				log.Println("checking for sources for", resource)

				resources := checker.CheckResource(resource)
				if len(resources) == 0 {
					break
				}

				log.Printf("found %d sources via %T", len(resources))

				resource = resources[len(resources)-1]

				if latestOnly {
					log.Printf("triggering latest via %T: %s\n", checker, resource)
					watchman.builder.Build(job, resource)
				} else {
					for i, resource := range resources {
						log.Printf("triggering %d of %d via %T: %s\n", i+1, len(resources), checker, resource)
						watchman.builder.Build(job, resource)
					}
				}
			}
		}
	}()
}

func (watchman *watchman) Stop() {
	close(watchman.stop)
	watchman.watching.Wait()
}
