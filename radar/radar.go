package radar

import (
	"errors"
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

	GetConfig() (atc.Config, db.ConfigVersion, bool, error)

	GetLatestVersionedResource(resource db.SavedResource) (db.SavedVersionedResource, bool, error)
	GetResource(resourceName string) (db.SavedResource, error)
	PauseResource(resourceName string) error
	UnpauseResource(resourceName string) error

	SaveResourceVersions(atc.ResourceConfig, []atc.Version) error
	SetResourceCheckError(resource db.SavedResource, err error) error
	LeaseResourceChecking(resource string, interval time.Duration, immediate bool) (db.Lease, bool, error)
}

type Radar struct {
	logger lager.Logger

	tracker resource.Tracker

	defaultInterval time.Duration

	db RadarDB
}

func NewRadar(
	tracker resource.Tracker,
	defaultInterval time.Duration,
	db RadarDB,
) *Radar {
	return &Radar{
		tracker:         tracker,
		defaultInterval: defaultInterval,
		db:              db,
	}
}

func (radar *Radar) Scanner(logger lager.Logger, resourceName string) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		// do an immediate initial check
		var interval time.Duration = 0

		close(ready)

		for {
			timer := time.NewTimer(interval)

			var resourceConfig atc.ResourceConfig

			select {
			case <-signals:
				return nil

			case <-timer.C:
				var err error

				resourceConfig, err = radar.getResourceConfig(logger, resourceName)
				if err != nil {
					return err
				}

				savedResource, err := radar.db.GetResource(resourceConfig.Name)
				if err != nil {
					return err
				}

				interval, err = radar.checkInterval(resourceConfig)
				if err != nil {
					setErr := radar.db.SetResourceCheckError(savedResource, err)
					if setErr != nil {
						logger.Error("failed-to-set-check-error", err)
					}

					return err
				}

				leaseLogger := logger.Session("lease", lager.Data{
					"resource": resourceName,
				})

				lease, leased, err := radar.db.LeaseResourceChecking(resourceName, interval, false)

				if err != nil {
					leaseLogger.Error("failed-to-get-lease", err, lager.Data{
						"resource": resourceName,
					})
					break
				}

				if !leased {
					leaseLogger.Debug("did-not-get-lease")
					break
				}

				err = radar.scan(logger.Session("tick"), resourceConfig, savedResource)

				lease.Break()

				if err != nil {
					return err
				}
			}
		}
	})
}

func (radar *Radar) Scan(logger lager.Logger, resourceName string) error {
	leaseLogger := logger.Session("lease", lager.Data{
		"resource": resourceName,
	})

	resourceConfig, err := radar.getResourceConfig(logger, resourceName)
	if err != nil {
		return err
	}

	savedResource, err := radar.db.GetResource(resourceConfig.Name)
	if err != nil {
		return err
	}

	interval, err := radar.checkInterval(resourceConfig)
	if err != nil {
		setErr := radar.db.SetResourceCheckError(savedResource, err)
		if setErr != nil {
			logger.Error("failed-to-set-check-error", err)
		}

		return err
	}

	for {
		lease, leased, err := radar.db.LeaseResourceChecking(resourceName, interval, true)
		if err != nil {
			leaseLogger.Error("failed-to-get-lease", err, lager.Data{
				"resource": resourceName,
			})

			return err
		}

		if !leased {
			leaseLogger.Debug("did-not-get-lease")
			time.Sleep(time.Second)
			continue
		}

		defer lease.Break()

		break
	}

	return radar.scan(logger, resourceConfig, savedResource)
}

func (radar *Radar) scan(logger lager.Logger, resourceConfig atc.ResourceConfig, savedResource db.SavedResource) error {
	pipelinePaused, err := radar.db.IsPaused()
	if err != nil {
		logger.Error("failed-to-check-if-pipeline-paused", err)
		return err
	}

	if pipelinePaused {
		logger.Debug("pipeline-paused")
		return nil
	}

	if savedResource.Paused {
		logger.Debug("resource-paused")
		return nil
	}

	typ := resource.ResourceType(resourceConfig.Type)

	res, err := radar.tracker.Init(
		logger,
		resource.EmptyMetadata{},
		checkIdentifier(radar.db.GetPipelineName(), resourceConfig),
		typ,
		[]string{},
		resource.VolumeMount{},
	)
	if err != nil {
		logger.Error("failed-to-initialize-new-resource", err)
		return err
	}

	defer res.Release()

	vr, found, err := radar.db.GetLatestVersionedResource(savedResource)
	if err != nil {
		logger.Error("failed-to-get-current-version", err)
		return err
	}

	var from db.Version
	if found {
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
		if rErr, ok := err.(resource.ErrResourceScriptFailed); ok {
			logger.Info("check-failed", lager.Data{"exit-status": rErr.ExitStatus})
			return nil
		}

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

func (radar *Radar) checkInterval(resourceConfig atc.ResourceConfig) (time.Duration, error) {
	interval := radar.defaultInterval
	if resourceConfig.CheckEvery != "" {
		configuredInterval, err := time.ParseDuration(resourceConfig.CheckEvery)
		if err != nil {
			return 0, err
		}

		interval = configuredInterval
	}

	return interval, nil
}

var errPipelineRemoved = errors.New("pipeline removed")

func (radar *Radar) getResourceConfig(logger lager.Logger, resourceName string) (atc.ResourceConfig, error) {
	config, _, found, err := radar.db.GetConfig()
	if err != nil {
		logger.Error("failed-to-get-config", err)
		return atc.ResourceConfig{}, err
	}

	if !found {
		logger.Info("pipeline-removed")
		return atc.ResourceConfig{}, errPipelineRemoved
	}

	resourceConfig, found := config.Resources.Lookup(resourceName)
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
