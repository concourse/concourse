package watchman

import (
	"sync"
	"time"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"

	"github.com/pivotal-golang/lager"
)

type Watchman interface {
	Watch(resource config.Resource)
	Stop()
}

type VersionDB interface {
	SaveVersionedResource(builds.VersionedResource) error
	GetLatestVersionedResource(string) (builds.VersionedResource, error)
}

type ResourceChecker interface {
	CheckResource(config.Resource, builds.Version) ([]builds.Version, error)
}

type watchman struct {
	logger lager.Logger

	checker  ResourceChecker
	tracker  VersionDB
	interval time.Duration

	stop     chan struct{}
	watching *sync.WaitGroup
}

func NewWatchman(
	logger lager.Logger,
	checker ResourceChecker,
	tracker VersionDB,
	interval time.Duration,
) Watchman {
	return &watchman{
		logger: logger,

		checker:  checker,
		tracker:  tracker,
		interval: interval,

		stop:     make(chan struct{}),
		watching: new(sync.WaitGroup),
	}
}

func (watchman *watchman) Watch(resource config.Resource) {
	watchman.watching.Add(1)

	go func() {
		defer watchman.watching.Done()

		ticker := time.NewTicker(watchman.interval)

		for {
			select {
			case <-watchman.stop:
				return

			case <-ticker.C:
				var from builds.Version

				if vr, err := watchman.tracker.GetLatestVersionedResource(resource.Name); err == nil {
					from = vr.Version
				}

				log := watchman.logger.Session("watchman", lager.Data{
					"resource": resource.Name,
					"from":     from,
				})

				log.Debug("check")

				newVersions, err := watchman.checker.CheckResource(resource, from)
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

				for _, version := range newVersions {
					err = watchman.tracker.SaveVersionedResource(builds.VersionedResource{
						Name:    resource.Name,
						Source:  resource.Source,
						Version: version,
					})
					if err != nil {
						log.Error("failed-to-save-current-version", err, lager.Data{
							"version": version,
						})
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
