package radar

import (
	"errors"
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/tedsuo/ifrit"

	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . VersionDB
type VersionDB interface {
	SaveVersionedResource(db.VersionedResource) (db.SavedVersionedResource, error)
	GetLatestVersionedResource(string) (db.SavedVersionedResource, error)
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
	tracker resource.Tracker,
	versionDB VersionDB,
	interval time.Duration,
	locker Locker,
	configDB ConfigDB,
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
	return &scanner{
		logger:       logger,
		resourceName: resourceName,

		tracker:   radar.tracker,
		versionDB: radar.versionDB,
		interval:  radar.interval,
		locker:    radar.locker,
		configDB:  radar.configDB,
	}
}

type scanner struct {
	logger lager.Logger

	resourceName string

	tracker   resource.Tracker
	versionDB VersionDB
	interval  time.Duration
	locker    Locker
	configDB  ConfigDB

	resource resource.Resource
}

func (scanner *scanner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	ticker := time.NewTicker(scanner.interval)

	close(ready)

	checkLock := []db.NamedLock{db.ResourceCheckingLock(scanner.resourceName)}

	for {
		select {
		case <-signals:
			return nil

		case <-ticker.C:
			resourceCheckingLock, err := scanner.locker.AcquireWriteLockImmediately(checkLock)
			if err != nil {
				continue
			}

			err = scanner.tick(scanner.logger.Session("tick"))
			if err != nil {
				resourceCheckingLock.Release()
				return err
			}

			resourceCheckingLock.Release()
		}
	}
}

var errResourceNoLongerConfigured = errors.New("resource no longer configured")

func (scanner *scanner) tick(logger lager.Logger) error {
	resLock := []db.NamedLock{db.ResourceLock(scanner.resourceName)}

	config, err := scanner.configDB.GetConfig()
	if err != nil {
		logger.Error("failed-to-get-config", err)
		// don't propagate error; we can just retry next tick
		return nil
	}

	resourceConfig, found := config.Resources.Lookup(scanner.resourceName)
	if !found {
		logger.Info("resource-removed-from-configuration")
		// return an error so that we exit
		return errResourceNoLongerConfigured
	}

	typ := resource.ResourceType(resourceConfig.Type)

	if scanner.resource == nil || scanner.resource.Type() != typ {
		if scanner.resource != nil {
			logger.Info("resource-type-changed", lager.Data{
				"before": typ,
				"after":  scanner.resource.Type(),
			})

			err := scanner.resource.Release()
			if err != nil {
				logger.Error("failed-to-release-checking-container", err)
				return err
			}
		}

		scanner.resource, err = scanner.tracker.Init(scanner.workerSessionID(resourceConfig), typ)
		if err != nil {
			logger.Error("failed-to-initialize-new-resource", err)
			return err
		}
	}

	lock, err := scanner.locker.AcquireReadLock(resLock)
	if err != nil {
		logger.Error("failed-to-acquire-inputs-lock", err)
		return nil
	}

	var from db.Version
	if vr, err := scanner.versionDB.GetLatestVersionedResource(scanner.resourceName); err == nil {
		from = vr.Version
	}

	lock.Release()

	logger.Debug("checking", lager.Data{
		"from": from,
	})

	newVersions, err := scanner.resource.Check(resourceConfig.Source, atc.Version(from))
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

	lock, err = scanner.locker.AcquireWriteLock(resLock)
	if err != nil {
		logger.Error("failed-to-acquire-inputs-lock", err)
		return nil
	}

	for _, version := range newVersions {
		_, err = scanner.versionDB.SaveVersionedResource(db.VersionedResource{
			Resource: resourceConfig.Name,
			Type:     resourceConfig.Type,
			Source:   db.Source(resourceConfig.Source),
			Version:  db.Version(version),
		})
		if err != nil {
			logger.Error("failed-to-save-current-version", err, lager.Data{
				"version": version,
			})
		}
	}

	lock.Release()

	return nil
}

func (scanner *scanner) workerSessionID(res atc.ResourceConfig) resource.SessionID {
	return resource.SessionID("check-" + res.Type + "-" + res.Name)
}
