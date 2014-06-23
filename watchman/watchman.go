package watchman

import (
	"fmt"
	"sync"
	"time"

	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/queue"
	"github.com/concourse/atc/resources"
	"github.com/garyburd/redigo/redis"
	"github.com/pivotal-golang/lager"
)

type Watchman interface {
	Watch(
		job config.Job,
		resource config.Resource,
		checker resources.Checker,
		eachVersion bool,
		interval time.Duration,
	)

	Stop()
}

type watchman struct {
	logger lager.Logger

	db     db.DB
	queuer queue.Queuer

	stop     chan struct{}
	watching *sync.WaitGroup
}

func NewWatchman(logger lager.Logger, db db.DB, queuer queue.Queuer) Watchman {
	return &watchman{
		logger: logger,

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
				from, err := watchman.db.GetCurrentVersion(job.Name, resource.Name)
				if err == redis.ErrNil {
					from = nil
				}

				watchman.logger.Info("watchman", "check", "", lager.Data{
					"job":      job.Name,
					"resource": resource.Name,
					"checker":  fmt.Sprintf("%T", checker),
					"from":     from,
				})

				newVersions, err := checker.CheckResource(resource, from)
				if err != nil {
					watchman.logger.Error("watchman", "check-failed", "", err)
					break
				}

				if len(newVersions) == 0 {
					break
				}

				watchman.logger.Info("watchman", "versions-found", "", lager.Data{
					"job":      job.Name,
					"resource": resource.Name,
					"checker":  fmt.Sprintf("%T", checker),
					"versions": newVersions,
					"total":    len(newVersions),
				})

				if eachVersion {
					for i, version := range newVersions {
						watchman.logger.Info(
							"watchman",
							"enqueue",
							fmt.Sprintf("%d of %d", i+1, len(newVersions)),
							lager.Data{
								"job":      job.Name,
								"resource": resource.Name,
								"version":  version,
								"checker":  fmt.Sprintf("%T", checker),
							},
						)

						watchman.queuer.Enqueue(job, resource, version)
					}
				} else {
					version := newVersions[len(newVersions)-1]

					watchman.logger.Info("watchman", "enqueue-latest", "", lager.Data{
						"job":      job.Name,
						"resource": resource.Name,
						"version":  version,
						"checker":  fmt.Sprintf("%T", checker),
					})

					watchman.queuer.Enqueue(job, resource, version)
				}
			}
		}
	}()
}

func (watchman *watchman) Stop() {
	close(watchman.stop)
	watchman.watching.Wait()
}
