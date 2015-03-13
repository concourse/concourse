package radar

import (
	"errors"
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
	"github.com/tedsuo/ifrit"

	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . VersionDB
type VersionDB interface {
	SaveResourceVersions(atc.ResourceConfig, []atc.Version) error
	GetLatestVersionedResource(string) (db.SavedVersionedResource, error)
}

var errResourceNoLongerConfigured = errors.New("resource no longer configured")

type Radar struct {
	logger lager.Logger

	tracker resource.Tracker

	interval time.Duration

	locker    Locker
	versionDB VersionDB
	configDB  db.ConfigDB
}

func NewRadar(
	tracker resource.Tracker,
	versionDB VersionDB,
	interval time.Duration,
	locker Locker,
	configDB db.ConfigDB,
) *Radar {
	return &Radar{
		tracker:   tracker,
		versionDB: versionDB,
		interval:  interval,
		locker:    locker,
		configDB:  configDB,
	}
}

func (radar *Radar) Scanner(logger lager.Logger, resourceName string) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		ticker := time.NewTicker(radar.interval)

		close(ready)

		for {
			select {
			case <-signals:
				return nil

			case <-ticker.C:
				resourceCheckingLock, err := radar.locker.AcquireWriteLockImmediately(radar.checkLock(resourceName))
				if err != nil {
					continue
				}

				err = radar.scan(logger.Session("tick"), resourceName)

				resourceCheckingLock.Release()

				if err != nil {
					return err
				}
			}
		}
	})
}

func (radar *Radar) Scan(logger lager.Logger, resourceName string) error {
	lock, err := radar.locker.AcquireWriteLock(radar.checkLock(resourceName))
	if err != nil {
		return err
	}

	defer lock.Release()

	return radar.scan(logger, resourceName)
}

func (radar *Radar) scan(logger lager.Logger, resourceName string) error {
	config, _, err := radar.configDB.GetConfig()
	if err != nil {
		logger.Error("failed-to-get-config", err)
		// don't propagate error; we can just retry next tick
		return nil
	}

	resourceConfig, found := config.Resources.Lookup(resourceName)
	if !found {
		logger.Info("resource-removed-from-configuration")
		// return an error so that we exit
		return errResourceNoLongerConfigured
	}

	typ := resource.ResourceType(resourceConfig.Type)

	res, err := radar.tracker.Init(checkIdentifier(resourceConfig), typ)
	if err != nil {
		logger.Error("failed-to-initialize-new-resource", err)
		return err
	}

	defer res.Release()

	var from db.Version
	if vr, err := radar.versionDB.GetLatestVersionedResource(resourceName); err == nil {
		from = vr.Version
	}

	logger.Debug("checking", lager.Data{
		"from": from,
	})

	newVersions, err := res.Check(resourceConfig.Source, atc.Version(from))
	if err != nil {
		logger.Error("failed-to-check", err)

		// ideally we'd check for non-recoverable errors like ErrContainerNotFound.
		// until Garden returns rich error objects, all we can do is exit.
		// [#85476532]

		return err
	}

	if len(newVersions) == 0 {
		return nil
	}

	logger.Info("versions-found", lager.Data{
		"versions": newVersions,
		"total":    len(newVersions),
	})

	err = radar.versionDB.SaveResourceVersions(resourceConfig, newVersions)
	if err != nil {
		logger.Error("failed-to-save-versions", err, lager.Data{
			"versions": newVersions,
		})
	}

	return nil
}

func (radar *Radar) checkLock(resourceName string) []db.NamedLock {
	return []db.NamedLock{db.ResourceCheckingLock(resourceName)}
}

func checkIdentifier(res atc.ResourceConfig) resource.Session {
	return resource.Session{
		ID: worker.Identifier{
			Name: res.Name,
			Type: "check",

			CheckType:   res.Type,
			CheckSource: res.Source,
		},
		Ephemeral: true,
	}
}
