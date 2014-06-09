package watchman

import (
	"log"
	"sync"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/winston-ci/winston/config"
	"github.com/winston-ci/winston/db"
	"github.com/winston-ci/winston/queue"
	"github.com/winston-ci/winston/resources"
)

type Watchman interface {
	Watch(
		job config.Job,
		resource config.Resource,
		checker resources.Checker,
		eachVersion bool,
		interval time.Duration,
		autoTrigger <-chan time.Time,
	)

	Stop()
}

type watchman struct {
	db     db.DB
	queuer queue.Queuer

	stop     chan struct{}
	watching *sync.WaitGroup
}

func NewWatchman(db db.DB, queuer queue.Queuer) Watchman {
	return &watchman{
		db:     db,
		queuer: queuer,

		stop:     make(chan struct{}),
		watching: new(sync.WaitGroup),
	}
}

func (watchman *watchman) Watch(
	job config.Job,
	resource config.Resource,
	checker resources.Checker,
	eachVersion bool,
	interval time.Duration,
	autoTrigger <-chan time.Time,
) {
	watchman.watching.Add(1)

	go func() {
		defer watchman.watching.Done()

		ticker := time.NewTicker(interval)

		for {
			select {
			case <-watchman.stop:
				return

			case <-autoTrigger:
				log.Printf("auto-triggering %s\n", job.Name)
				watchman.queuer.Trigger(job)

			case <-ticker.C:
				from, err := watchman.db.GetCurrentVersion(job.Name, resource.Name)
				if err == redis.ErrNil {
					from = nil
				}

				log.Printf("checking for sources for %s via %T from %s since %v\n", job.Name, checker, resource, from)

				newVersions := checker.CheckResource(resource, from)
				if len(newVersions) == 0 {
					break
				}

				log.Printf("found %d new versions for %s via %T", len(newVersions), job.Name, checker)

				if eachVersion {
					for i, version := range newVersions {
						log.Printf("triggering %s (%d of %d) via %T: %s\n", job.Name, i+1, len(newVersions), checker, version)
						watchman.queuer.Enqueue(job, resource, version)
					}
				} else {
					log.Printf("triggering %s (latest) via %T: %s\n", job.Name, checker, resource)
					watchman.queuer.Enqueue(job, resource, newVersions[len(newVersions)-1])
				}
			}
		}
	}()
}

func (watchman *watchman) Stop() {
	close(watchman.stop)
	watchman.watching.Wait()
}
