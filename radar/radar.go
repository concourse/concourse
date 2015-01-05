package radar

import (
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/exec/resource"
	"github.com/tedsuo/ifrit"

	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . VersionDB
type VersionDB interface {
	SaveVersionedResource(db.VersionedResource) error
	GetLatestVersionedResource(string) (db.VersionedResource, error)
}

//go:generate counterfeiter . ConfigDB
type ConfigDB interface {
	GetConfig() (atc.Config, error)
}

type Radar struct {
	logger lager.Logger

	tracker resource.Tracker

	interval time.Duration

	locker    Locker
	versionDB VersionDB
	configDB  ConfigDB
}

func NewRadar(
	logger lager.Logger,
	tracker resource.Tracker,
	versionDB VersionDB,
	interval time.Duration,
	locker Locker,
	configDB ConfigDB,
) *Radar {
	return &Radar{
		logger: logger,

		tracker: tracker,

		versionDB: versionDB,
		interval:  interval,

		locker: locker,

		configDB: configDB,
	}
}

func (radar *Radar) Scan(resourceName string) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		ticker := time.NewTicker(radar.interval)

		close(ready)

		var res resource.Resource

		defer func() {
			if res != nil {
				res.Release()
			}
		}()

		checkLock := []db.NamedLock{db.ResourceCheckingLock(resourceName)}
		resLock := []db.NamedLock{db.ResourceLock(resourceName)}

		var resourceCheckingLock db.Lock

		defer func() {
			if resourceCheckingLock != nil {
				resourceCheckingLock.Release()
			}
		}()

		for {
			var err error

			select {
			case <-signals:
				return nil

			case <-ticker.C:
				resourceCheckingLock, err = radar.locker.AcquireWriteLockImmediately(checkLock)
				if err != nil {
					break
				}

				config, err := radar.configDB.GetConfig()
				if err != nil {
					break
				}

				resourceConfig, found := config.Resources.Lookup(resourceName)
				if !found {
					return nil
				}

				typ := resource.ResourceType(resourceConfig.Type)

				if res == nil || res.Type() != typ {
					if res != nil {
						err := res.Release()
						if err != nil {
							return err
						}
					}

					res, err = radar.tracker.Init("", typ)
					if err != nil {
						return err
					}
				}

				log := radar.logger.Session("radar", lager.Data{
					"resource": resourceConfig.Name,
					"type":     resourceConfig.Type,
				})

				lock, err := radar.locker.AcquireReadLock(resLock)
				if err != nil {
					log.Error("failed-to-acquire-inputs-lock", err)
					break
				}

				var from db.Version
				if vr, err := radar.versionDB.GetLatestVersionedResource(resourceName); err == nil {
					from = vr.Version
				}

				lock.Release()

				log.Debug("check", lager.Data{
					"from": from,
				})

				newVersions, err := res.Check(resourceConfig.Source, atc.Version(from))
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

				lock, err = radar.locker.AcquireWriteLock(resLock)
				if err != nil {
					log.Error("failed-to-acquire-inputs-lock", err)
					break
				}

				for _, version := range newVersions {
					err = radar.versionDB.SaveVersionedResource(db.VersionedResource{
						Resource: resourceConfig.Name,
						Type:     resourceConfig.Type,
						Source:   db.Source(resourceConfig.Source),
						Version:  db.Version(version),
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
	})
}
