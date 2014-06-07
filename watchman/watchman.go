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
		latestOnly bool,
		interval time.Duration,
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
	latestOnly bool,
	interval time.Duration,
) {
	watchman.watching.Add(1)

	go func() {
		defer watchman.watching.Done()

		ticker := time.NewTicker(interval)

		var triggers <-chan time.Time
		if job.TriggerEvery != 0 {
			triggerTicker := time.NewTicker(time.Duration(job.TriggerEvery))
			triggers = triggerTicker.C
		}

		for {
			select {
			case <-watchman.stop:
				return

			case <-triggers:
				log.Printf("triggering %s via %s interval\n", job.Name, time.Duration(job.TriggerEvery))
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

				if latestOnly {
					log.Printf("triggering %s (latest) via %T: %s\n", job.Name, checker, resource)
					watchman.queuer.Enqueue(job, resource, newVersions[len(newVersions)-1])
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
