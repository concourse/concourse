package radar

import (
	"fmt"
	"os"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
	"github.com/tedsuo/ifrit"

	"github.com/pivotal-golang/lager"
)

type resourceNotConfiguredError struct {
	ResourceName string
}

func (err resourceNotConfiguredError) Error() string {
	return fmt.Sprintf("resource '%s' was not found in config", err.ResourceName)
}

//go:generate counterfeiter . RadarDB

type RadarDB interface {
	GetPipelineName() string
	ScopedName(string) string

	IsPaused() (bool, error)

	GetConfig() (atc.Config, db.ConfigVersion, error)

	GetLatestVersionedResource(resource db.SavedResource) (db.SavedVersionedResource, error)
	GetResource(resourceName string) (db.SavedResource, error)
	PauseResource(resourceName string) error
	UnpauseResource(resourceName string) error

	SaveResourceVersions(atc.ResourceConfig, []atc.Version) error
	SetResourceCheckError(resource db.SavedResource, err error) error
}

type Radar struct {
	logger lager.Logger

	tracker resource.Tracker

	interval time.Duration

	locker Locker
	db     RadarDB
}

func NewRadar(
	tracker resource.Tracker,
	interval time.Duration,
	locker Locker,
	db RadarDB,
) *Radar {
	return &Radar{
		tracker:  tracker,
		interval: interval,
		locker:   locker,
		db:       db,
	}
}

func (radar *Radar) Scanner(logger lager.Logger, resourceName string) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		var ticker *time.Ticker
		resourceConfig, err := radar.getResourceConfig(logger, resourceName)
		if err != nil {
			return err
		}

		if resourceConfig.CheckEvery != 0 {
			ticker = time.NewTicker(time.Duration(resourceConfig.CheckEvery))
		} else {
			ticker = time.NewTicker(radar.interval)
		}

		close(ready)

		for {
			select {
			case <-signals:
				return nil

			case <-ticker.C:
				lock := radar.checkLock(radar.db.ScopedName(resourceName))
				resourceCheckingLock, err := radar.locker.AcquireWriteLockImmediately(lock)

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
	lock, err := radar.locker.AcquireWriteLock(radar.checkLock(radar.db.ScopedName(resourceName)))
	if err != nil {
		return err
	}

	defer lock.Release()

	return radar.scan(logger, resourceName)
}

func (radar *Radar) scan(logger lager.Logger, resourceName string) error {
	pipelinePaused, err := radar.db.IsPaused()
	if err != nil {
		logger.Error("failed-to-check-if-pipeline-paused", err)
		return err
	}

	if pipelinePaused {
		logger.Debug("pipeline-paused")
		return nil
	}

	savedResource, err := radar.db.GetResource(resourceName)
	if err != nil {
		return err
	}

	if savedResource.Paused {
		return nil
	}

	resourceConfig, err := radar.getResourceConfig(logger, resourceName)
	if err != nil {
		return err
	}

	typ := resource.ResourceType(resourceConfig.Type)

	res, err := radar.tracker.Init(checkIdentifier(radar.db.GetPipelineName(), resourceConfig), typ, []string{})
	if err != nil {
		logger.Error("failed-to-initialize-new-resource", err)
		return err
	}

	defer res.Release()

	var from db.Version
	if vr, err := radar.db.GetLatestVersionedResource(savedResource); err == nil {
		from = vr.Version
	}

	logger.Debug("checking", lager.Data{
		"from": from,
	})

	newVersions, err := res.Check(resourceConfig.Source, atc.Version(from))
	setErr := radar.db.SetResourceCheckError(savedResource, err)
	if setErr != nil {
		logger.Error("failed-to-set-check-error", err)
	}

	if err != nil {
		logger.Error("failed-to-check", err)

		return err
	}

	if len(newVersions) == 0 {
		logger.Debug("no-new-versions")
		return nil
	}

	logger.Info("versions-found", lager.Data{
		"versions": newVersions,
		"total":    len(newVersions),
	})

	err = radar.db.SaveResourceVersions(resourceConfig, newVersions)
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

func (radar *Radar) getResourceConfig(logger lager.Logger, resourceName string) (atc.ResourceConfig, error) {
	var found bool
	var resourceConfig atc.ResourceConfig

	config, _, err := radar.db.GetConfig()
	if err != nil {
		logger.Error("failed-to-get-config", err)
		// don't propagate error; we can just retry next tick
		return resourceConfig, err
	}

	resourceConfig, found = config.Resources.Lookup(resourceName)
	if !found {
		logger.Info("resource-removed-from-configuration")
		// return an error so that we exit
		return resourceConfig, resourceNotConfiguredError{ResourceName: resourceName}
	}

	return resourceConfig, nil
}

func checkIdentifier(pipelineName string, res atc.ResourceConfig) resource.Session {
	return resource.Session{
		ID: worker.Identifier{
			PipelineName: pipelineName,

			Name: res.Name,
			Type: "check",

			CheckType:   res.Type,
			CheckSource: res.Source,
		},
		Ephemeral: true,
	}
}
