package radar

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/concourse/concourse/atc/runtime"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/worker"
)

var GlobalResourceCheckTimeout time.Duration

type resourceScanner struct {
	clock                 clock.Clock
	pool                  worker.Pool
	resourceConfigFactory db.ResourceConfigFactory
	defaultInterval       time.Duration
	dbPipeline            db.Pipeline
	externalURL           string
	secrets               creds.Secrets
	varSourcePool         creds.VarSourcePool
	strategy              worker.ContainerPlacementStrategy
}

func NewResourceScanner(
	clock clock.Clock,
	pool worker.Pool,
	resourceConfigFactory db.ResourceConfigFactory,
	defaultInterval time.Duration,
	dbPipeline db.Pipeline,
	externalURL string,
	secrets creds.Secrets,
	varSourcePool creds.VarSourcePool,
	strategy worker.ContainerPlacementStrategy,
) Scanner {
	return &resourceScanner{
		clock:                 clock,
		pool:                  pool,
		resourceConfigFactory: resourceConfigFactory,
		defaultInterval:       defaultInterval,
		dbPipeline:            dbPipeline,
		externalURL:           externalURL,
		secrets:               secrets,
		varSourcePool:         varSourcePool,
		strategy:              strategy,
	}
}

var ErrFailedToAcquireLock = errors.New("failed to acquire lock")
var ErrResourceTypeNotFound = errors.New("resource type not found")
var ErrResourceTypeCheckError = errors.New("resource type failed to check")

func (scanner *resourceScanner) Run(logger lager.Logger, resourceID int) (time.Duration, error) {
	interval, err := scanner.scan(logger.Session("tick"), resourceID, nil, false, false)

	err = swallowErrResourceScriptFailed(err)

	return interval, err
}

func (scanner *resourceScanner) ScanFromVersion(logger lager.Logger, resourceID int, fromVersion atc.Version) error {
	_, err := scanner.scan(logger, resourceID, fromVersion, true, true)

	return err
}

func (scanner *resourceScanner) Scan(logger lager.Logger, resourceID int) error {
	_, err := scanner.scan(logger, resourceID, nil, true, false)

	err = swallowErrResourceScriptFailed(err)

	return err
}

func (scanner *resourceScanner) scan(logger lager.Logger, resourceID int, fromVersion atc.Version, mustComplete bool, saveGiven bool) (time.Duration, error) {
	savedResource, found, err := scanner.dbPipeline.ResourceByID(resourceID)
	if err != nil {
		return 0, err
	}

	if !found {
		logger.Debug("resource-not-found")
		return 0, db.ResourceNotFoundError{ID: resourceID}
	}

	lockLogger := logger.Session("lock", lager.Data{
		"resource": savedResource.Name(),
	})

	timeout, err := scanner.parseResourceCheckTimeoutOrDefault(savedResource.CheckTimeout())
	if err != nil {
		scanner.setResourceCheckError(logger, savedResource, err)
		logger.Error("failed-to-read-check-timeout", err)
		return 0, err
	}

	interval, err := scanner.checkInterval(savedResource.CheckEvery())
	if err != nil {
		scanner.setResourceCheckError(logger, savedResource, err)
		logger.Error("failed-to-read-check-interval", err)
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
		if parentType.Name() != savedResource.Type() {
			continue
		}

		for {
			if parentType.Version() != nil {
				break
			}

			if parentType.CheckError() != nil {
				scanner.setResourceCheckError(logger, savedResource, parentType.CheckError())
				logger.Error("resource-type-failed-to-check", err, lager.Data{"resource-type": parentType.Name()})
				return 0, ErrResourceTypeCheckError
			} else {
				logger.Debug("waiting-on-resource-type-version", lager.Data{"resource-type": parentType.Name()})
				scanner.clock.Sleep(10 * time.Second)

				found, err := parentType.Reload()
				if err != nil {
					logger.Error("failed-to-reload-resource-type", err, lager.Data{"resource-type": parentType.Name()})
					return 0, err
				}

				if !found {
					logger.Error("resource-type-not-found", err, lager.Data{"resource-type": parentType.Name()})
					return 0, ErrResourceTypeNotFound
				}
			}
		}
	}

	resourceTypes, err = scanner.dbPipeline.ResourceTypes()
	if err != nil {
		logger.Error("failed-to-get-resource-types", err)
		return 0, err
	}

	// Combine pipeline specific var_sources with the global credential manager.
	varss, err := scanner.dbPipeline.Variables(logger, scanner.secrets, scanner.varSourcePool)
	if err != nil {
		logger.Error("failed-to-create-variables", err)
		scanner.setResourceCheckError(logger, savedResource, err)
		return 0, err
	}

	versionedResourceTypes, err := creds.NewVersionedResourceTypes(
		varss,
		resourceTypes.Deserialize(),
	).Evaluate()
	if err != nil {
		logger.Error("failed-to-evaluate-resource-types", err)
		scanner.setResourceCheckError(logger, savedResource, err)
		return 0, err
	}

	source, err := creds.NewSource(varss, savedResource.Source()).Evaluate()
	if err != nil {
		logger.Error("failed-to-evaluate-resource-source", err)
		scanner.setResourceCheckError(logger, savedResource, err)
		return 0, err
	}

	resourceConfigScope, err := savedResource.SetResourceConfig(
		source,
		versionedResourceTypes,
	)
	if err != nil {
		logger.Error("failed-to-set-resource-config-id-on-resource", err)
		scanner.setResourceCheckError(logger, savedResource, err)
		return 0, err
	}

	// Clear out check error on the resource
	scanner.setResourceCheckError(logger, savedResource, nil)

	currentVersion := savedResource.CurrentPinnedVersion()
	if currentVersion != nil {
		_, found, err := resourceConfigScope.FindVersion(currentVersion)

		if err != nil {
			logger.Error("failed-to-find-pinned-version-on-resource", err, lager.Data{"pinned-version": currentVersion})
			chkErr := resourceConfigScope.SetCheckError(err)
			if chkErr != nil {
				logger.Error("failed-to-set-check-error-on-resource-config", chkErr)
			}
			return 0, err
		}
		if found {
			logger.Info("skipping-check-because-pinned-version-found", lager.Data{"pinned-version": currentVersion})
			return interval, nil
		}

		fromVersion = currentVersion
	}

	for {
		lock, acquired, err := resourceConfigScope.AcquireResourceCheckingLock(
			logger,
		)
		if err != nil {
			lockLogger.Error("failed-to-get-lock", err, lager.Data{
				"resource_name":   savedResource.Name(),
				"resource_config": resourceConfigScope.ResourceConfig().ID(),
			})
			return interval, ErrFailedToAcquireLock
		}

		if !acquired {
			lockLogger.Debug("did-not-get-lock")
			scanner.clock.Sleep(time.Second)
			continue
		}

		defer lock.Release()

		updated, err := resourceConfigScope.UpdateLastCheckStartTime(interval, mustComplete)
		if err != nil {
			return interval, err
		}

		if !updated {
			logger.Debug("interval-not-reached", lager.Data{
				"interval": interval,
			})
			return interval, ErrFailedToAcquireLock
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
		savedResource,
		resourceConfigScope,
		fromVersion,
		versionedResourceTypes,
		source,
		saveGiven,
		timeout,
	)
}

func (scanner *resourceScanner) check(
	logger lager.Logger,
	savedResource db.Resource,
	resourceConfigScope db.ResourceConfigScope,
	fromVersion atc.Version,
	resourceTypes atc.VersionedResourceTypes,
	source atc.Source,
	saveGiven bool,
	timeout time.Duration,
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

	found, err := scanner.dbPipeline.Reload()
	if err != nil {
		logger.Error("failed-to-reload-scannerdb", err)
		return err
	}
	if !found {
		logger.Info("pipeline-removed")
		return errPipelineRemoved
	}

	metadata := resource.TrackerMetadata{
		ResourceName: savedResource.Name(),
		PipelineName: savedResource.PipelineName(),
		ExternalURL:  scanner.externalURL,
	}

	containerSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: savedResource.Type(),
		},
		BindMounts: []worker.BindMountSource{
			&worker.CertsVolumeMount{Logger: logger},
		},
		Tags:   savedResource.Tags(),
		TeamID: scanner.dbPipeline.TeamID(),
		Env:    metadata.Env(),
	}

	workerSpec := worker.WorkerSpec{
		ResourceType:  savedResource.Type(),
		Tags:          savedResource.Tags(),
		ResourceTypes: resourceTypes,
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
		logger.Error("failed-to-choose-a-worker", err)
		chkErr := resourceConfigScope.SetCheckError(err)
		if chkErr != nil {
			logger.Error("failed-to-set-check-error-on-resource-config", chkErr)
		}
		return err
	}

	container, err := chosenWorker.FindOrCreateContainer(
		context.Background(),
		logger,
		worker.NoopImageFetchingDelegate{},
		owner,
		db.ContainerMetadata{
			Type: db.ContainerTypeCheck,
		},
		containerSpec,
		resourceTypes,
	)
	if err != nil {
		// TODO: remove this after ephemeral check containers.
		// Sometimes we pass in a check session thats too close to
		// expirey into FindOrCreateContainer such that the container
		// gced before the call is completed
		if err == worker.ResourceConfigCheckSessionExpiredError {
			return nil
		}
		logger.Error("failed-to-create-or-find-container", err)
		chkErr := resourceConfigScope.SetCheckError(err)
		if chkErr != nil {
			logger.Error("failed-to-set-check-error-on-resource-config", chkErr)
		}
		return err
	}

	logger.Debug("checking", lager.Data{
		"from": fromVersion,
	})

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	//TODO: Check if we need to add anything else to this processSpec
	processSpec := runtime.ProcessSpec{
		Path: "/opt/resource/check",
	}
	res := resource.NewResource(source, nil, fromVersion)
	newVersions, err := res.Check(ctx, processSpec, container)
	if err == context.DeadlineExceeded {
		err = fmt.Errorf("Timed out after %v while checking for new versions - perhaps increase your resource check timeout?", timeout)
	}

	resourceConfigScope.SetCheckError(err)
	metric.ResourceCheck{
		PipelineName: scanner.dbPipeline.Name(),
		ResourceName: savedResource.Name(),
		TeamName:     scanner.dbPipeline.TeamName(),
		Success:      err == nil,
	}.Emit(logger)

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
	} else {
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
	}

	updated, err := resourceConfigScope.UpdateLastCheckEndTime()
	if err != nil {
		return err
	}

	if !updated {
		logger.Debug("did-not-update-last-check-finished")
	}

	return nil
}

func swallowErrResourceScriptFailed(err error) error {
	if _, ok := err.(runtime.ErrResourceScriptFailed); ok {
		return nil
	}
	return err
}

func (scanner *resourceScanner) parseResourceCheckTimeoutOrDefault(checkTimeout string) (time.Duration, error) {
	interval := GlobalResourceCheckTimeout
	if checkTimeout != "" {
		configuredInterval, err := time.ParseDuration(checkTimeout)
		if err != nil {
			return 0, err
		}

		interval = configuredInterval
	}

	return interval, nil
}

func (scanner *resourceScanner) checkInterval(checkEvery string) (time.Duration, error) {
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

func (scanner *resourceScanner) setResourceCheckError(logger lager.Logger, savedResource db.Resource, err error) {
	setErr := savedResource.SetCheckSetupError(err)
	if setErr != nil {
		logger.Error("failed-to-set-check-error", err)
	}
}

var errPipelineRemoved = errors.New("pipeline removed")
