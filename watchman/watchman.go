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

				log := watchman.logger.Session("watchman", lager.Data{
					"job":      job.Name,
					"resource": resource.Name,
					"checker":  fmt.Sprintf("%T", checker),
					"from":     from,
				})

				log.Debug("check")

				newVersions, err := checker.CheckResource(resource, from)
				if err != nil {
					log.Error("failed-to-check", err)
					break
				}

				if len(newVersions) == 0 {
					break
				}

				log.Info("versions-found", lager.Data{
					"versions": newVersions,
					"total":    len(newVersions),
				})

				latestVersion := newVersions[len(newVersions)-1]

				err = watchman.db.SaveCurrentVersion(job.Name, resource.Name, latestVersion)
				if err != nil {
					log.Error("failed-to-save-current-version", err, lager.Data{
						"version": latestVersion,
					})
				}

				if eachVersion {
					for i, version := range newVersions {
						log.Info("enqueue", lager.Data{
							"which":   fmt.Sprintf("%d of %d", i+1, len(newVersions)),
							"version": version,
						})

						watchman.queuer.Enqueue(job, resource, version)
					}
				} else {
					log.Info("enqueue-latest", lager.Data{
						"version": latestVersion,
					})

					watchman.queuer.Enqueue(job, resource, latestVersion)
				}
			}
		}
	}()
}

func (watchman *watchman) Stop() {
	close(watchman.stop)
	watchman.watching.Wait()
}
