package radar

import (
	"errors"
	"reflect"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

type resourceScanner struct {
	clock           clock.Clock
	tracker         resource.Tracker
	defaultInterval time.Duration
	db              RadarDB
	externalURL     string
}

func NewResourceScanner(
	clock clock.Clock,
	tracker resource.Tracker,
	defaultInterval time.Duration,
	db RadarDB,
	externalURL string,
) Scanner {
	return &resourceScanner{
		clock:           clock,
		tracker:         tracker,
		defaultInterval: defaultInterval,
		db:              db,
		externalURL:     externalURL,
	}
}

var ErrFailedToAcquireLease = errors.New("failed-to-acquire-lock")

func (scanner *resourceScanner) Run(logger lager.Logger, resourceName string) (time.Duration, error) {
	resourceConfig, resourceTypes, err := scanner.getResourceConfig(logger, resourceName)
	if err != nil {
		return 0, err
	}

	savedResource, found, err := scanner.db.GetResource(resourceConfig.Name)
	if err != nil {
		return 0, err
	}

	if !found {
		return 0, db.ResourceNotFoundError{Name: resourceConfig.Name}
	}

	interval, err := scanner.checkInterval(resourceConfig)
	if err != nil {
		setErr := scanner.db.SetResourceCheckError(savedResource, err)
		if setErr != nil {
			logger.Error("failed-to-set-check-error", err)
		}

		return 0, err
	}

	lockLogger := logger.Session("lock", lager.Data{
		"resource": resourceName,
	})

	lock, acquired, err := scanner.db.AcquireResourceCheckingLock(logger, savedResource, interval, false)

	if err != nil {
		lockLogger.Error("failed-to-get-lock", err, lager.Data{
			"resource": resourceName,
		})
		return interval, ErrFailedToAcquireLease
	}

	if !acquired {
		lockLogger.Debug("did-not-get-lock")
		return interval, ErrFailedToAcquireLease
	}

	defer lock.Release()

	vr, _, err := scanner.db.GetLatestVersionedResource(resourceName)
	if err != nil {
		logger.Error("failed-to-get-current-version", err)
		return interval, err
	}

	err = swallowErrResourceScriptFailed(
		scanner.scan(logger.Session("tick"), resourceConfig, resourceTypes, savedResource, atc.Version(vr.Version)),
	)
	if err != nil {
		return interval, err
	}

	return interval, nil
}

func (scanner *resourceScanner) ScanFromVersion(logger lager.Logger, resourceName string, fromVersion atc.Version) error {
	// if fromVersion is nil then force a check without specifying a version
	// otherwise specify fromVersion to underlying call to resource.Check()
	lockLogger := logger.Session("lock", lager.Data{
		"resource": resourceName,
	})

	savedResource, found, err := scanner.db.GetResource(resourceName)
	if err != nil {
		return err
	}

	if !found {
		logger.Debug("resource-not-found")
		return db.ResourceNotFoundError{Name: resourceName}
	}

	resourceConfig, resourceTypes, err := scanner.getResourceConfig(logger, resourceName)
	if err != nil {
		return err
	}

	interval, err := scanner.checkInterval(resourceConfig)
	if err != nil {
		setErr := scanner.db.SetResourceCheckError(savedResource, err)
		if setErr != nil {
			logger.Error("failed-to-set-check-error", err)
		}

		return err
	}

	for {
		lock, acquired, err := scanner.db.AcquireResourceCheckingLock(logger, savedResource, interval, true)
		if err != nil {
			lockLogger.Error("failed-to-get-lock", err, lager.Data{
				"resource": resourceName,
			})

			return err
		}

		if !acquired {
			lockLogger.Debug("did-not-get-lock")
			scanner.clock.Sleep(time.Second)
			continue
		}

		defer lock.Release()

		break
	}

	return scanner.scan(logger, resourceConfig, resourceTypes, savedResource, fromVersion)
}

func (scanner *resourceScanner) Scan(logger lager.Logger, resourceName string) error {
	vr, _, err := scanner.db.GetLatestVersionedResource(resourceName)
	if err != nil {
		logger.Error("failed-to-get-current-version", err)
		return err
	}

	return swallowErrResourceScriptFailed(
		scanner.ScanFromVersion(logger, resourceName, atc.Version(vr.Version)),
	)
}

func (scanner *resourceScanner) scan(
	logger lager.Logger,
	resourceConfig atc.ResourceConfig,
	resourceTypes atc.ResourceTypes,
	savedResource db.SavedResource,
	fromVersion atc.Version,
) error {
	pipelinePaused, err := scanner.db.IsPaused()
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

	pipelineID := scanner.db.GetPipelineID()

	var resourceTypeVersion atc.Version
	_, found := resourceTypes.Lookup(resourceConfig.Type)
	if found {
		savedResourceType, resourceTypeFound, err := scanner.db.GetResourceType(resourceConfig.Type)
		if err != nil {
			logger.Error("failed-to-find-resource-type", err)
			return err
		}
		if resourceTypeFound {
			resourceTypeVersion = atc.Version(savedResourceType.Version)
		}
	}

	session := resource.Session{
		ID: worker.Identifier{
			ResourceTypeVersion: resourceTypeVersion,
			ResourceID:          savedResource.ID,
			Stage:               db.ContainerStageRun,
			CheckType:           resourceConfig.Type,
			CheckSource:         resourceConfig.Source,
		},
		Metadata: worker.Metadata{
			Type:       db.ContainerTypeCheck,
			PipelineID: pipelineID,
			TeamID:     scanner.db.TeamID(),
		},
		Ephemeral: true,
	}

	res, err := scanner.tracker.Init(
		logger,
		resource.TrackerMetadata{
			ResourceName: resourceConfig.Name,
			PipelineName: savedResource.PipelineName,
			ExternalURL:  scanner.externalURL,
		},
		session,
		resource.ResourceType(resourceConfig.Type),
		[]string{},
		scanner.db.TeamID(),
		resourceTypes,
		worker.NoopImageFetchingDelegate{},
	)
	if err != nil {
		logger.Error("failed-to-initialize-new-resource", err)
		return err
	}

	defer res.Release(nil)

	logger.Debug("checking", lager.Data{
		"from": fromVersion,
	})

	newVersions, err := res.Check(resourceConfig.Source, fromVersion)

	setErr := scanner.db.SetResourceCheckError(savedResource, err)
	if setErr != nil {
		logger.Error("failed-to-set-check-error", err)
	}

	if err != nil {
		if rErr, ok := err.(resource.ErrResourceScriptFailed); ok {
			logger.Info("check-failed", lager.Data{"exit-status": rErr.ExitStatus})
			return rErr
		}

		logger.Error("failed-to-check", err)
		return err
	}

	if len(newVersions) == 0 || reflect.DeepEqual(newVersions, []atc.Version{fromVersion}) {
		logger.Debug("no-new-versions")
		return nil
	}

	logger.Info("versions-found", lager.Data{
		"versions": newVersions,
		"total":    len(newVersions),
	})

	err = scanner.db.SaveResourceVersions(resourceConfig, newVersions)
	if err != nil {
		logger.Error("failed-to-save-versions", err, lager.Data{
			"versions": newVersions,
		})
	}

	return nil
}

func swallowErrResourceScriptFailed(err error) error {
	if _, ok := err.(resource.ErrResourceScriptFailed); ok {
		return nil
	}
	return err
}

func (scanner *resourceScanner) checkInterval(resourceConfig atc.ResourceConfig) (time.Duration, error) {
	interval := scanner.defaultInterval
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

func (scanner *resourceScanner) getResourceConfig(logger lager.Logger, resourceName string) (atc.ResourceConfig, atc.ResourceTypes, error) {
	config, _, found, err := scanner.db.GetConfig()
	if err != nil {
		logger.Error("failed-to-get-config", err)
		return atc.ResourceConfig{}, nil, err
	}

	if !found {
		logger.Info("pipeline-removed")
		return atc.ResourceConfig{}, nil, errPipelineRemoved
	}

	resourceConfig, found := config.Resources.Lookup(resourceName)
	if !found {
		logger.Info("resource-removed-from-configuration")
		return resourceConfig, nil, ResourceNotConfiguredError{ResourceName: resourceName}
	}

	return resourceConfig, config.ResourceTypes, nil
}
