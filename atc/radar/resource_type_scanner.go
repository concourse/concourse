package radar

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
)

type resourceTypeScanner struct {
	clock                 clock.Clock
	pool                  worker.Pool
	resourceFactory       resource.ResourceFactory
	resourceConfigFactory db.ResourceConfigFactory
	defaultInterval       time.Duration
	dbPipeline            db.Pipeline
	externalURL           string
	secrets               creds.Secrets
	varSourcePool         creds.VarSourcePool
	strategy              worker.ContainerPlacementStrategy
}

func NewResourceTypeScanner(
	clock clock.Clock,
	pool worker.Pool,
	resourceFactory resource.ResourceFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	defaultInterval time.Duration,
	dbPipeline db.Pipeline,
	externalURL string,
	secrets creds.Secrets,
	varSourcePool creds.VarSourcePool,
	strategy worker.ContainerPlacementStrategy,
) Scanner {
	return &resourceTypeScanner{
		clock:                 clock,
		pool:                  pool,
		resourceFactory:       resourceFactory,
		resourceConfigFactory: resourceConfigFactory,
		defaultInterval:       defaultInterval,
		dbPipeline:            dbPipeline,
		externalURL:           externalURL,
		secrets:               secrets,
		varSourcePool:         varSourcePool,
		strategy:              strategy,
	}
}

func (scanner *resourceTypeScanner) Run(logger lager.Logger, resourceTypeID int) (time.Duration, error) {
	return scanner.scan(logger.Session("tick"), resourceTypeID, nil, false, false)
}

func (scanner *resourceTypeScanner) ScanFromVersion(logger lager.Logger, resourceTypeID int, fromVersion atc.Version) error {
	_, err := scanner.scan(logger, resourceTypeID, fromVersion, true, true)
	return err
}

func (scanner *resourceTypeScanner) Scan(logger lager.Logger, resourceTypeID int) error {
	_, err := scanner.scan(logger, resourceTypeID, nil, true, false)
	return err
}

func (scanner *resourceTypeScanner) scan(logger lager.Logger, resourceTypeID int, fromVersion atc.Version, mustComplete bool, saveGiven bool) (time.Duration, error) {
	savedResourceType, found, err := scanner.dbPipeline.ResourceTypeByID(resourceTypeID)
	if err != nil {
		logger.Error("failed-to-find-resource-type-in-db", err)
		return 0, err
	}

	if !found {
		return 0, db.ResourceTypeNotFoundError{ID: resourceTypeID}
	}

	lockLogger := logger.Session("lock", lager.Data{
		"resource-type": savedResourceType.Name(),
	})

	interval, err := scanner.checkInterval(savedResourceType.CheckEvery())
	if err != nil {
		scanner.setCheckError(logger, savedResourceType, err)
		return 0, err
	}

	found, err = scanner.dbPipeline.Reload()
	if !found {
		return 0, fmt.Errorf("pipeline %s(%d) not found", scanner.dbPipeline.Name(), scanner.dbPipeline.ID())
	}
	if err != nil {
		return 0, fmt.Errorf("failed to reload pipeline %s(%d)", scanner.dbPipeline.Name(), scanner.dbPipeline.ID())
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

		if err = scanner.Scan(logger, parentType.ID()); err != nil {
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

	varss, err := scanner.dbPipeline.Variables(logger, scanner.secrets, scanner.varSourcePool)
	if err != nil {
		return 0, err
	}

	versionedResourceTypes, err := creds.NewVersionedResourceTypes(
		varss,
		resourceTypes.Deserialize(),
	).Evaluate()
	if err != nil {
		logger.Error("failed-to-evaluate-resource-types", err)
		scanner.setCheckError(logger, savedResourceType, err)
		return 0, err
	}

	source, err := creds.NewSource(varss, savedResourceType.Source()).Evaluate()
	if err != nil {
		logger.Error("failed-to-evaluate-resource-type-source", err)
		scanner.setCheckError(logger, savedResourceType, err)
		return 0, err
	}

	resourceConfigScope, err := savedResourceType.SetResourceConfig(
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
		lock, acquired, err := resourceConfigScope.AcquireResourceCheckingLock(
			logger,
		)
		if err != nil {
			lockLogger.Error("failed-to-get-lock", err, lager.Data{
				"resource-type":      savedResourceType.Name(),
				"resource-config-id": resourceConfigScope.ResourceConfig().ID(),
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

		updated, err := resourceConfigScope.UpdateLastCheckStartTime(interval, mustComplete)
		if err != nil {
			lockLogger.Error("failed-to-get-update-last-checked", err, lager.Data{
				"resource-type":      savedResourceType.Name(),
				"resource-config-id": resourceConfigScope.ResourceConfig().ID(),
			})
			return interval, ErrFailedToAcquireLock
		}

		if !updated {
			lockLogger.Debug("did-not-update-last-checked")
			if mustComplete {
				scanner.clock.Sleep(time.Second)
				continue
			} else {
				return interval, ErrFailedToAcquireLock
			}
		}

		break
	}

	if fromVersion == nil {
		rcv, found, err := resourceConfigScope.LatestVersion()
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
		resourceConfigScope,
		fromVersion,
		versionedResourceTypes,
		source,
		saveGiven,
	)
}

func (scanner *resourceTypeScanner) check(
	logger lager.Logger,
	savedResourceType db.ResourceType,
	resourceConfigScope db.ResourceConfigScope,
	fromVersion atc.Version,
	versionedResourceTypes atc.VersionedResourceTypes,
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
		BindMounts: []worker.BindMountSource{
			&worker.CertsVolumeMount{Logger: logger},
		},
	}

	workerSpec := worker.WorkerSpec{
		ResourceType:  savedResourceType.Type(),
		Tags:          savedResourceType.Tags(),
		ResourceTypes: versionedResourceTypes.Without(savedResourceType.Name()),
		TeamID:        scanner.dbPipeline.TeamID(),
	}

	owner := db.NewResourceConfigCheckSessionContainerOwner(
		resourceConfigScope.ResourceConfig().ID(),
		resourceConfigScope.ResourceConfig().OriginBaseResourceType().ID,
		ContainerExpiries,
	)

	chosenWorker, err := scanner.pool.FindOrChooseWorkerForContainer(
		context.Background(),
		logger,
		owner,
		containerSpec,
		workerSpec,
		scanner.strategy,
	)
	if err != nil {
		chkErr := resourceConfigScope.SetCheckError(err)
		if chkErr != nil {
			logger.Error("failed-to-set-check-error-on-resource-config", chkErr)
		}
		logger.Error("failed-to-find-or-choose-worker", err)
		return err
	}

	container, err := chosenWorker.FindOrCreateContainer(
		context.Background(),
		logger,
		worker.NoopImageFetchingDelegate{},
		db.NewResourceConfigCheckSessionContainerOwner(
			resourceConfigScope.ResourceConfig().ID(),
			resourceConfigScope.ResourceConfig().OriginBaseResourceType().ID,
			ContainerExpiries,
		),
		db.ContainerMetadata{
			Type: db.ContainerTypeCheck,
		},
		containerSpec,
		versionedResourceTypes.Without(savedResourceType.Name()),
	)
	if err != nil {
		chkErr := resourceConfigScope.SetCheckError(err)
		if chkErr != nil {
			logger.Error("failed-to-set-check-error-on-resource-config", chkErr)
		}
		logger.Error("failed-to-create-or-find-container", err)
		return err
	}

	//TODO: Check if we need to add anything else to this processSpec
	processSpec := runtime.ProcessSpec{
		Path: "/opt/resource/check",
	}

	res := scanner.resourceFactory.NewResource(source, nil, fromVersion)
	newVersions, err := res.Check(context.TODO(), processSpec, container)
	resourceConfigScope.SetCheckError(err)
	if err != nil {
		if rErr, ok := err.(runtime.ErrResourceScriptFailed); ok {
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

	err = resourceConfigScope.SaveVersions(newVersions)
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
	setErr := savedResourceType.SetCheckSetupError(err)
	if setErr != nil {
		logger.Error("failed-to-set-check-error", err)
	}
}
