package radar

import (
	"context"
	"reflect"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/worker"
)

type resourceTypeScanner struct {
	clock                 clock.Clock
	resourceFactory       resource.ResourceFactory
	resourceConfigFactory db.ResourceConfigFactory
	defaultInterval       time.Duration
	dbPipeline            db.Pipeline
	externalURL           string
	variables             creds.Variables
	containerExpiries     db.ContainerOwnerExpiries
}

func NewResourceTypeScanner(
	clock clock.Clock,
	resourceFactory resource.ResourceFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	defaultInterval time.Duration,
	dbPipeline db.Pipeline,
	externalURL string,
	variables creds.Variables,
	containerExpiries db.ContainerOwnerExpiries,
) Scanner {
	return &resourceTypeScanner{
		clock:                 clock,
		resourceFactory:       resourceFactory,
		resourceConfigFactory: resourceConfigFactory,
		defaultInterval:       defaultInterval,
		dbPipeline:            dbPipeline,
		externalURL:           externalURL,
		variables:             variables,
		containerExpiries:     containerExpiries,
	}
}

func (scanner *resourceTypeScanner) Run(logger lager.Logger, resourceTypeName string) (time.Duration, error) {
	return scanner.scan(logger.Session("tick"), resourceTypeName, nil, false, false)
}

func (scanner *resourceTypeScanner) ScanFromVersion(logger lager.Logger, resourceTypeName string, fromVersion atc.Version) error {
	_, err := scanner.scan(logger, resourceTypeName, fromVersion, true, true)
	return err
}

func (scanner *resourceTypeScanner) Scan(logger lager.Logger, resourceTypeName string) error {
	_, err := scanner.scan(logger, resourceTypeName, nil, true, false)
	return err
}

func (scanner *resourceTypeScanner) scan(logger lager.Logger, resourceTypeName string, fromVersion atc.Version, mustComplete bool, saveGiven bool) (time.Duration, error) {
	lockLogger := logger.Session("lock", lager.Data{
		"resource-type": resourceTypeName,
	})

	savedResourceType, found, err := scanner.dbPipeline.ResourceType(resourceTypeName)
	if err != nil {
		logger.Error("failed-to-find-resource-type-in-db", err)
		return 0, err
	}

	if !found {
		return 0, db.ResourceTypeNotFoundError{Name: resourceTypeName}
	}

	interval, err := scanner.checkInterval(savedResourceType.CheckEvery())
	if err != nil {
		scanner.setCheckError(logger, savedResourceType, err)
		return 0, err
	}

	resourceTypes, err := scanner.dbPipeline.ResourceTypes()
	if err != nil {
		logger.Error("failed-to-get-resource-types", err)
		return 0, err
	}

	for _, parentType := range resourceTypes {
		if parentType.Name() == savedResourceType.Name() {
			continue
		}
		if parentType.Name() != savedResourceType.Type() {
			continue
		}
		if parentType.Version() != nil {
			continue
		}

		if err = scanner.Scan(logger, parentType.Name()); err != nil {
			logger.Error("failed-to-scan-parent-resource-type-version", err)
			scanner.setCheckError(logger, savedResourceType, err)
			return 0, err
		}
	}

	resourceTypes, err = scanner.dbPipeline.ResourceTypes()
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
		scanner.setCheckError(logger, savedResourceType, err)
		return 0, err
	}

	resourceConfig, err := savedResourceType.SetResourceConfig(
		logger,
		source,
		versionedResourceTypes.Without(savedResourceType.Name()),
	)
	if err != nil {
		logger.Error("failed-to-set-resource-config-id-on-resource-type", err)
		scanner.setCheckError(logger, savedResourceType, err)
		return 0, err
	}

	// Clear out the check error on the resource type
	scanner.setCheckError(logger, savedResourceType, err)

	reattempt := true
	for reattempt {
		reattempt = mustComplete
		lock, acquired, err := resourceConfig.AcquireResourceConfigCheckingLockWithIntervalCheck(
			logger,
			interval,
			mustComplete,
		)
		if err != nil {
			lockLogger.Error("failed-to-get-lock", err, lager.Data{
				"resource-type":      resourceTypeName,
				"resource-config-id": resourceConfig.ID(),
			})
			return interval, ErrFailedToAcquireLock
		}

		if !acquired {
			lockLogger.Debug("did-not-get-lock")
			if mustComplete {
				scanner.clock.Sleep(time.Second)
				continue
			} else {
				return interval, ErrFailedToAcquireLock
			}
		}

		defer lock.Release()

		break
	}

	if fromVersion == nil {
		rcv, found, err := resourceConfig.LatestVersion()
		if err != nil {
			logger.Error("failed-to-get-current-version", err)
			return interval, err
		}

		if found {
			fromVersion = atc.Version(rcv.Version())
		}
	}

	return interval, scanner.check(
		logger,
		savedResourceType,
		resourceConfig,
		fromVersion,
		versionedResourceTypes,
		source,
		saveGiven,
	)
}

func (scanner *resourceTypeScanner) check(
	logger lager.Logger,
	savedResourceType db.ResourceType,
	resourceConfig db.ResourceConfig,
	fromVersion atc.Version,
	versionedResourceTypes creds.VersionedResourceTypes,
	source atc.Source,
	saveGiven bool,
) error {
	pipelinePaused, err := scanner.dbPipeline.CheckPaused()
	if err != nil {
		logger.Error("failed-to-check-if-pipeline-paused", err)
		return err
	}

	if pipelinePaused {
		logger.Debug("pipeline-paused")
		return nil
	}

	containerSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: savedResourceType.Type(),
		},
		Tags:   savedResourceType.Tags(),
		TeamID: scanner.dbPipeline.TeamID(),
	}

	workerSpec := worker.WorkerSpec{
		ResourceType:  savedResourceType.Type(),
		Tags:          savedResourceType.Tags(),
		ResourceTypes: versionedResourceTypes.Without(savedResourceType.Name()),
	}

	res, err := scanner.resourceFactory.NewResource(
		context.Background(),
		logger,
		db.NewResourceConfigCheckSessionContainerOwner(resourceConfig, scanner.containerExpiries),
		db.ContainerMetadata{
			Type: db.ContainerTypeCheck,
		},
		containerSpec,
		workerSpec,
		versionedResourceTypes.Without(savedResourceType.Name()),
		worker.NoopImageFetchingDelegate{},
	)
	if err != nil {
		logger.Error("failed-to-initialize-new-container", err)

		if err == worker.ErrNoGlobalWorkers {
			err = atc.ErrNoWorkers
		}

		chkErr := resourceConfig.SetCheckError(err)
		if chkErr != nil {
			logger.Error("failed-to-set-check-error-on-resource-config", chkErr)
		}

		return err
	}

	newVersions, err := res.Check(context.TODO(), source, fromVersion)
	resourceConfig.SetCheckError(err)
	if err != nil {
		if rErr, ok := err.(resource.ErrResourceScriptFailed); ok {
			logger.Info("check-failed", lager.Data{"exit-status": rErr.ExitStatus})
			return rErr
		}

		logger.Error("failed-to-check", err)
		return err
	}

	if len(newVersions) == 0 || (!saveGiven && reflect.DeepEqual(newVersions, []atc.Version{fromVersion})) {
		logger.Debug("no-new-versions")
		return nil
	}

	logger.Info("versions-found", lager.Data{
		"versions": newVersions,
		"total":    len(newVersions),
	})

	err = resourceConfig.SaveVersions(newVersions)
	if err != nil {
		logger.Error("failed-to-save-resource-config-versions", err, lager.Data{
			"versions": newVersions,
		})
		return err
	}

	return nil
}

func (scanner *resourceTypeScanner) checkInterval(checkEvery string) (time.Duration, error) {
	interval := scanner.defaultInterval
	if checkEvery != "" {
		configuredInterval, err := time.ParseDuration(checkEvery)
		if err != nil {
			return 0, err
		}

		interval = configuredInterval
	}

	return interval, nil
}

func (scanner *resourceTypeScanner) setCheckError(logger lager.Logger, savedResourceType db.ResourceType, err error) {
	setErr := savedResourceType.SetCheckError(err)
	if setErr != nil {
		logger.Error("failed-to-set-check-error", err)
	}
}
