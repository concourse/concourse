package radar

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

type resourceTypeScanner struct {
	resourceFactory                   resource.ResourceFactory
	resourceConfigCheckSessionFactory db.ResourceConfigCheckSessionFactory
	defaultInterval                   time.Duration
	dbPipeline                        db.Pipeline
	externalURL                       string
	variables                         creds.Variables
}

func NewResourceTypeScanner(
	resourceFactory resource.ResourceFactory,
	resourceConfigCheckSessionFactory db.ResourceConfigCheckSessionFactory,
	defaultInterval time.Duration,
	dbPipeline db.Pipeline,
	externalURL string,
	variables creds.Variables,
) Scanner {
	return &resourceTypeScanner{
		resourceFactory:                   resourceFactory,
		resourceConfigCheckSessionFactory: resourceConfigCheckSessionFactory,
		defaultInterval:                   defaultInterval,
		dbPipeline:                        dbPipeline,
		externalURL:                       externalURL,
		variables:                         variables,
	}
}

func (scanner *resourceTypeScanner) Run(logger lager.Logger, resourceTypeName string) (time.Duration, error) {
	pipelinePaused, err := scanner.dbPipeline.CheckPaused()
	if err != nil {
		logger.Error("failed-to-check-if-pipeline-paused", err)
		return 0, err
	}

	if pipelinePaused {
		logger.Debug("pipeline-paused")
		return scanner.defaultInterval, nil
	}

	lockLogger := logger.Session("lock", lager.Data{
		"resource-type": resourceTypeName,
	})

	savedResourceType, found, err := scanner.dbPipeline.ResourceType(resourceTypeName)
	if err != nil {
		logger.Error("failed-to-get-current-version", err)
		return 0, err
	}

	if !found {
		return 0, db.ResourceTypeNotFoundError{Name: resourceTypeName}
	}

	resourceTypes, err := scanner.dbPipeline.ResourceTypes()
	if err != nil {
		logger.Error("failed-to-get-resource-types", err)
		return 0, err
	}

	versionedResourceTypes := creds.NewVersionedResourceTypes(
		scanner.variables,
		resourceTypes.Deserialize(),
	)

	source, err := creds.NewSource(scanner.variables, savedResourceType.Source()).Evaluate()
	if err != nil {
		logger.Error("failed-to-evaluate-resource-type-source", err)
		return 0, err
	}

	resourceConfigCheckSession, err := scanner.resourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSession(
		logger,
		savedResourceType.Type(),
		source,
		versionedResourceTypes.Without(savedResourceType.Name()),
		ContainerExpiries,
	)
	if err != nil {
		logger.Error("failed-to-find-or-create-resource-config", err)
		return 0, err
	}

	err = savedResourceType.SetResourceConfig(resourceConfigCheckSession.ResourceConfig().ID)
	if err != nil {
		logger.Error("failed-to-set-resource-config-id-on-resource-type", err)
		return 0, err
	}

	lock, acquired, err := scanner.dbPipeline.AcquireResourceTypeCheckingLockWithIntervalCheck(logger, resourceTypeName, resourceConfigCheckSession.ResourceConfig(), scanner.defaultInterval, false)
	if err != nil {
		lockLogger.Error("failed-to-get-lock", err, lager.Data{
			"resource-type": resourceTypeName,
		})
		return scanner.defaultInterval, ErrFailedToAcquireLock
	}

	if !acquired {
		lockLogger.Debug("did-not-get-lock")
		return scanner.defaultInterval, ErrFailedToAcquireLock
	}

	defer lock.Release()

	err = scanner.resourceTypeScan(logger.Session("tick"), resourceTypeName, savedResourceType, resourceConfigCheckSession, versionedResourceTypes, source)
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

func (scanner *resourceTypeScanner) resourceTypeScan(logger lager.Logger, resourceTypeName string, savedResourceType db.ResourceType, resourceConfigCheckSession db.ResourceConfigCheckSession, versionedResourceTypes creds.VersionedResourceTypes, source atc.Source) error {
	resourceSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: savedResourceType.Type(),
		},
		Tags:   []string{},
		TeamID: scanner.dbPipeline.TeamID(),
	}

	res, err := scanner.resourceFactory.NewResource(
		logger,
		nil,
		db.NewResourceConfigCheckSessionContainerOwner(resourceConfigCheckSession, scanner.dbPipeline.TeamID()),
		db.ContainerMetadata{
			Type: db.ContainerTypeCheck,
		},
		resourceSpec,
		versionedResourceTypes.Without(savedResourceType.Name()),
		worker.NoopImageFetchingDelegate{},
	)
	if err != nil {
		logger.Error("failed-to-initialize-new-container", err)
		return err
	}

	newVersions, err := res.Check(source, atc.Version(savedResourceType.Version()))
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
	err = savedResourceType.SaveVersion(version)
	if err != nil {
		logger.Error("failed-to-save-resource-type-version", err, lager.Data{
			"version": version,
		})
		return err
	}

	return nil
}
