package radar

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

type resourceTypeScanner struct {
	tracker         resource.Tracker
	defaultInterval time.Duration
	db              RadarDB
	externalURL     string
}

func NewResourceTypeScanner(
	tracker resource.Tracker,
	defaultInterval time.Duration,
	db RadarDB,
	externalURL string,
) Scanner {
	return &resourceTypeScanner{
		tracker:         tracker,
		defaultInterval: defaultInterval,
		db:              db,
		externalURL:     externalURL,
	}
}

func (scanner *resourceTypeScanner) Run(logger lager.Logger, resourceTypeName string) (time.Duration, error) {
	pipelinePaused, err := scanner.db.IsPaused()
	if err != nil {
		logger.Error("failed-to-check-if-pipeline-paused", err)
		return 0, err
	}

	if pipelinePaused {
		logger.Debug("pipeline-paused")
		return scanner.defaultInterval, nil
	}

	savedResourceType, found, err := scanner.db.GetResourceType(resourceTypeName)
	if err != nil {
		logger.Error("failed-to-get-current-version", err)
		return 0, err
	}

	if !found {
		return 0, db.ResourceTypeNotFoundError{Name: resourceTypeName}
	}

	lockLogger := logger.Session("lock", lager.Data{
		"resource-type": resourceTypeName,
	})

	lock, acquired, err := scanner.db.AcquireResourceTypeCheckingLock(logger, savedResourceType, scanner.defaultInterval, false)
	if err != nil {
		lockLogger.Error("failed-to-get-lock", err, lager.Data{
			"resource-type": resourceTypeName,
		})
		return scanner.defaultInterval, ErrFailedToAcquireLease
	}

	if !acquired {
		lockLogger.Debug("did-not-get-lock")
		return scanner.defaultInterval, ErrFailedToAcquireLease
	}

	defer lock.Release()

	configResourceType, err := scanner.getResourceTypeConfig(logger, resourceTypeName)
	if err != nil {
		return 0, err
	}

	err = scanner.resourceTypeScan(logger.Session("tick"), configResourceType, savedResourceType.Version)
	if err != nil {
		return 0, err
	}

	return scanner.defaultInterval, nil
}

func (scanner *resourceTypeScanner) Scan(logger lager.Logger, resourceTypeName string) error {
	return nil
}

func (scanner *resourceTypeScanner) ScanFromVersion(logger lager.Logger, resourceTypeName string, fromVersion atc.Version) error {
	return nil
}

func (scanner *resourceTypeScanner) resourceTypeScan(logger lager.Logger, resourceType atc.ResourceType, fromVersion db.Version) error {
	pipelineID := scanner.db.GetPipelineID()

	session := resource.Session{
		ID: worker.Identifier{
			Stage:               db.ContainerStageCheck,
			CheckType:           resourceType.Type,
			CheckSource:         resourceType.Source,
			ImageResourceType:   resourceType.Type,
			ImageResourceSource: resourceType.Source,
		},
		Metadata: worker.Metadata{
			Type:                 db.ContainerTypeCheck,
			PipelineID:           pipelineID,
			WorkingDirectory:     "",
			EnvironmentVariables: nil,
		},
		Ephemeral: true,
	}

	res, err := scanner.tracker.Init(
		logger.Session("check-image"),
		resource.EmptyMetadata{},
		session,
		resource.ResourceType(resourceType.Type),
		[]string{},
		scanner.db.TeamID(),
		atc.ResourceTypes{},
		worker.NoopImageFetchingDelegate{},
	)
	if err != nil {
		return err
	}

	defer res.Release(nil)

	logger.Debug("checking")

	newVersions, err := res.Check(resourceType.Source, atc.Version(fromVersion))
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

	version := newVersions[len(newVersions)-1]
	err = scanner.db.SaveResourceTypeVersion(resourceType, version)
	if err != nil {
		logger.Error("failed-to-save-resource-type-version", err, lager.Data{
			"version": version,
		})
		return err
	}

	return nil
}

func (scanner *resourceTypeScanner) getResourceTypeConfig(logger lager.Logger, resourceTypeName string) (atc.ResourceType, error) {
	config, _, found, err := scanner.db.GetConfig()
	if err != nil {
		logger.Error("failed-to-get-config", err)
		return atc.ResourceType{}, err
	}

	if !found {
		logger.Info("pipeline-removed")
		return atc.ResourceType{}, errPipelineRemoved
	}

	resourceType, found := config.ResourceTypes.Lookup(resourceTypeName)
	if !found {
		logger.Info("resource-type-removed-from-configuration")
		return resourceType, ResourceNotConfiguredError{ResourceName: resourceTypeName}
	}

	return resourceType, nil
}
