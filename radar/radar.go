package radar

import (
	"os"
	"sync"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/tedsuo/ifrit"

	"github.com/pivotal-golang/lager"
)

type VersionDB interface {
	SaveVersionedResource(db.VersionedResource) error
	GetLatestVersionedResource(string) (db.VersionedResource, error)
}

type ResourceChecker interface {
	CheckResource(atc.ResourceConfig, db.Version) ([]db.Version, error)
}

type ConfigDB interface {
	GetConfig() (atc.Config, error)
}

type Radar struct {
	logger lager.Logger

	tracker  VersionDB
	interval time.Duration

	locker Locker

	configDB ConfigDB

	stop     chan struct{}
	scanning *sync.WaitGroup

	failing  map[string]bool
	checking map[string]bool
	statusL  *sync.Mutex
}

func NewRadar(
	logger lager.Logger,
	tracker VersionDB,
	interval time.Duration,
	locker Locker,
	configDB ConfigDB,
) *Radar {
	return &Radar{
		logger: logger,

		tracker:  tracker,
		interval: interval,

		locker: locker,

		configDB: configDB,

		stop:     make(chan struct{}),
		scanning: new(sync.WaitGroup),

		failing:  make(map[string]bool),
		checking: make(map[string]bool),
		statusL:  new(sync.Mutex),
	}
}

func (radar *Radar) Scan(checker ResourceChecker, resourceName string) ifrit.Process {
	radar.scanning.Add(1)

	return ifrit.Invoke(ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		defer radar.scanning.Done()

		ticker := time.NewTicker(radar.interval)

		close(ready)

		for {
			var resourceCheckingLock db.Lock
			var err error
			select {
			case <-radar.stop:
				return nil

			case <-ticker.C:
				resourceCheckingLock, err = radar.locker.AcquireWriteLockImmediately([]db.NamedLock{db.ResourceCheckingLock(resourceName)})
				if err != nil {
					break
				}

				config, err := radar.configDB.GetConfig()
				if err != nil {
					continue
				}

				resource, found := config.Resources.Lookup(resourceName)
				if !found {
					return nil
				}

				radar.setChecking(resource.Name)

				var from db.Version
				log := radar.logger.Session("radar", lager.Data{
					"type":     resource.Type,
					"resource": resource.Name,
					"from":     from,
				})

				lock, err := radar.locker.AcquireReadLock([]db.NamedLock{db.ResourceLock(resource.Name)})
				if err != nil {
					log.Error("failed-to-acquire-inputs-lock", err, lager.Data{
						"resource_name": resource.Name,
					})
					break
				}
				if vr, err := radar.tracker.GetLatestVersionedResource(resource.Name); err == nil {
					from = vr.Version
				}
				lock.Release()

				log.Debug("check")

				newVersions, err := checker.CheckResource(resource, from)

				radar.setFailing(resource.Name, err != nil)

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

				lock, err = radar.locker.AcquireWriteLock([]db.NamedLock{db.ResourceLock(resource.Name)})
				if err != nil {
					log.Error("failed-to-acquire-inputs-lock", err, lager.Data{
						"resource_name": resource.Name,
					})
					break
				}

				for _, version := range newVersions {
					err = radar.tracker.SaveVersionedResource(db.VersionedResource{
						Name:    resource.Name,
						Type:    resource.Type,
						Source:  db.Source(resource.Source),
						Version: version,
					})
					if err != nil {
						log.Error("failed-to-save-current-version", err, lager.Data{
							"version": version,
						})
					}
				}

				lock.Release()
			}

			if resourceCheckingLock != nil {
				resourceCheckingLock.Release()
			}
		}
	}))
}

func (radar *Radar) ResourceStatus(resource string) (bool, bool) {
	radar.statusL.Lock()
	defer radar.statusL.Unlock()
	return radar.failing[resource], radar.checking[resource]
}

func (radar *Radar) Stop() {
	close(radar.stop)
	radar.scanning.Wait()
}

func (radar *Radar) setChecking(resource string) {
	radar.statusL.Lock()
	radar.checking[resource] = true
	radar.statusL.Unlock()
}

func (radar *Radar) setFailing(resource string, failing bool) {
	radar.statusL.Lock()

	delete(radar.checking, resource)

	if failing {
		radar.failing[resource] = true
	} else {
		delete(radar.failing, resource)
	}

	radar.statusL.Unlock()
}
