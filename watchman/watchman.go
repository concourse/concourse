package watchman

import (
	"log"
	"sync"
	"time"

	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/config"
	"github.com/winston-ci/winston/queue"
	"github.com/winston-ci/winston/resources"
)

type Watchman interface {
	Watch(
		job config.Job,
		resource config.Resource,
		from builds.Version,
		checker resources.Checker,
		latestOnly bool,
		interval time.Duration,
	)

	Stop()
}

type watchman struct {
	queuer queue.Queuer

	stop     chan struct{}
	watching *sync.WaitGroup
}

func NewWatchman(queuer queue.Queuer) Watchman {
	return &watchman{
		queuer: queuer,

		stop:     make(chan struct{}),
		watching: new(sync.WaitGroup),
	}
}

func (watchman *watchman) Watch(
	job config.Job,
	resource config.Resource,
	from builds.Version,
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
				log.Printf("checking for sources for %s via %T from %s since %v\n", job.Name, checker, resource, from)

				newVersions := checker.CheckResource(resource, from)
				if len(newVersions) == 0 {
					break
				}

				log.Printf("found %d new versions for %s via %T", len(newVersions), job.Name, checker)

				from = newVersions[len(newVersions)-1]

				if latestOnly {
					log.Printf("triggering %s (latest) via %T: %s\n", job.Name, checker, resource)
					watchman.queuer.Enqueue(job, resource, from)
				} else {
					for i, version := range newVersions {
						log.Printf("triggering %s (%d of %d) via %T: %s\n", job.Name, i+1, len(newVersions), checker, version)
						watchman.queuer.Enqueue(job, resource, version)
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
