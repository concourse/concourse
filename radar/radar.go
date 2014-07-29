package radar

import (
	"sync"
	"time"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"

	"github.com/pivotal-golang/lager"
)

type VersionDB interface {
	SaveVersionedResource(builds.VersionedResource) error
	GetLatestVersionedResource(string) (builds.VersionedResource, error)
}

type ResourceChecker interface {
	CheckResource(config.Resource, builds.Version) ([]builds.Version, error)
}

type Radar struct {
	logger lager.Logger

	checker  ResourceChecker
	tracker  VersionDB
	interval time.Duration

	stop     chan struct{}
	scanning *sync.WaitGroup
}

func NewRadar(
	logger lager.Logger,
	checker ResourceChecker,
	tracker VersionDB,
	interval time.Duration,
) *Radar {
	return &Radar{
		logger: logger,

		checker:  checker,
		tracker:  tracker,
		interval: interval,

		stop:     make(chan struct{}),
		scanning: new(sync.WaitGroup),
	}
}

func (radar *Radar) Scan(resource config.Resource) {
	radar.scanning.Add(1)

	go func() {
		defer radar.scanning.Done()

		ticker := time.NewTicker(radar.interval)

		for {
			select {
			case <-radar.stop:
				return

			case <-ticker.C:
				var from builds.Version

				if vr, err := radar.tracker.GetLatestVersionedResource(resource.Name); err == nil {
					from = vr.Version
				}

				log := radar.logger.Session("radar", lager.Data{
					"resource": resource.Name,
					"from":     from,
				})

				log.Debug("check")

				newVersions, err := radar.checker.CheckResource(resource, from)
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
					err = radar.tracker.SaveVersionedResource(builds.VersionedResource{
						Name:    resource.Name,
						Type:    resource.Type,
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

func (radar *Radar) Stop() {
	close(radar.stop)
	radar.scanning.Wait()
}
