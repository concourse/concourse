package radar

import (
	"errors"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
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

var ErrFailedToAcquireLease = errors.New("failed-to-acquire-lease")

func (scanner *resourceScanner) Run(logger lager.Logger, resourceName string) (time.Duration, error) {
	resourceConfig, resourceTypes, err := scanner.getResourceConfig(logger, resourceName)
	if err != nil {
		return 0, err
	}

	savedResource, err := scanner.db.GetResource(resourceConfig.Name)
	if err != nil {
		return 0, err
	}

	interval, err := scanner.checkInterval(resourceConfig)
	if err != nil {
		setErr := scanner.db.SetResourceCheckError(savedResource, err)
		if setErr != nil {
			logger.Error("failed-to-set-check-error", err)
		}

		return 0, err
	}

	leaseLogger := logger.Session("lease", lager.Data{
		"resource": resourceName,
	})

	lease, leased, err := scanner.db.LeaseResourceChecking(logger, resourceName, interval, false)

	if err != nil {
		leaseLogger.Error("failed-to-get-lease", err, lager.Data{
			"resource": resourceName,
		})
		return interval, ErrFailedToAcquireLease
	}

	if !leased {
		leaseLogger.Debug("did-not-get-lease")
		return interval, ErrFailedToAcquireLease
	}

	err = scanner.scan(logger.Session("tick"), resourceConfig, resourceTypes, savedResource)

	lease.Break()

	if err != nil {
		return interval, err
	}

	return interval, nil
}

func (scanner *resourceScanner) Scan(logger lager.Logger, resourceName string) error {
	leaseLogger := logger.Session("lease", lager.Data{
		"resource": resourceName,
	})

	resourceConfig, resourceTypes, err := scanner.getResourceConfig(logger, resourceName)
	if err != nil {
		return err
	}

	savedResource, err := scanner.db.GetResource(resourceConfig.Name)
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
		lease, leased, err := scanner.db.LeaseResourceChecking(logger, resourceName, interval, true)
		if err != nil {
			leaseLogger.Error("failed-to-get-lease", err, lager.Data{
				"resource": resourceName,
			})

			return err
		}

		if !leased {
			leaseLogger.Debug("did-not-get-lease")
			scanner.clock.Sleep(time.Second)
			continue
		}

		defer lease.Break()

		break
	}

	return scanner.scan(logger, resourceConfig, resourceTypes, savedResource)
}

func (scanner *resourceScanner) scan(logger lager.Logger, resourceConfig atc.ResourceConfig, resourceTypes atc.ResourceTypes, savedResource db.SavedResource) error {
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

	vr, found, err := scanner.db.GetLatestVersionedResource(savedResource)
	if err != nil {
		logger.Error("failed-to-get-current-version", err)
		return err
	}

	var from db.Version
	if found {
		from = vr.Version
	}

	pipelineName := scanner.db.GetPipelineName()

	var resourceTypeVersion atc.Version
	_, found = resourceTypes.Lookup(resourceConfig.Type)
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
			Type:         db.ContainerTypeCheck,
			PipelineName: pipelineName,
		},
		Ephemeral: true,
	}

	res, err := scanner.tracker.Init(
		logger,
		resource.TrackerMetadata{
			ResourceName: resourceConfig.Name,
			PipelineName: pipelineName,
			ExternalURL:  scanner.externalURL,
		},
		session,
		resource.ResourceType(resourceConfig.Type),
		[]string{},
		resourceTypes,
		worker.NoopImageFetchingDelegate{},
	)
	if err != nil {
		logger.Error("failed-to-initialize-new-resource", err)
		return err
	}

	defer res.Release(nil)

	logger.Debug("checking", lager.Data{
		"from": from,
	})

	newVersions, err := res.Check(resourceConfig.Source, atc.Version(from))

	setErr := scanner.db.SetResourceCheckError(savedResource, err)
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

	err = scanner.db.SaveResourceVersions(resourceConfig, newVersions)
	if err != nil {
		logger.Error("failed-to-save-versions", err, lager.Data{
			"versions": newVersions,
		})
	}

	return nil
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
