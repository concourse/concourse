package radar

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

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
	resourceFactory       resource.ResourceFactory
	resourceConfigFactory db.ResourceConfigFactory
	defaultInterval       time.Duration
	dbPipeline            db.Pipeline
	externalURL           string
	variables             creds.Variables
	typeScanner           Scanner
}

func NewResourceScanner(
	clock clock.Clock,
	resourceFactory resource.ResourceFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	defaultInterval time.Duration,
	dbPipeline db.Pipeline,
	externalURL string,
	variables creds.Variables,
	typeScanner Scanner,
) Scanner {
	return &resourceScanner{
		clock:                 clock,
		resourceFactory:       resourceFactory,
		resourceConfigFactory: resourceConfigFactory,
		defaultInterval:       defaultInterval,
		dbPipeline:            dbPipeline,
		externalURL:           externalURL,
		variables:             variables,
		typeScanner:           typeScanner,
	}
}

var ErrFailedToAcquireLock = errors.New("failed to acquire lock")

func (scanner *resourceScanner) Run(logger lager.Logger, resourceName string) (time.Duration, error) {
	interval, err := scanner.scan(logger.Session("tick"), resourceName, nil, false, false)

	err = swallowErrResourceScriptFailed(err)

	return interval, err
}

func (scanner *resourceScanner) ScanFromVersion(logger lager.Logger, resourceName string, fromVersion atc.Version) error {
	_, err := scanner.scan(logger, resourceName, fromVersion, true, true)

	return err
}

func (scanner *resourceScanner) Scan(logger lager.Logger, resourceName string) error {
	_, err := scanner.scan(logger, resourceName, nil, true, false)

	err = swallowErrResourceScriptFailed(err)

	return err
}

func (scanner *resourceScanner) scan(logger lager.Logger, resourceName string, fromVersion atc.Version, mustComplete bool, saveGiven bool) (time.Duration, error) {
	lockLogger := logger.Session("lock", lager.Data{
		"resource": resourceName,
	})

	savedResource, found, err := scanner.dbPipeline.Resource(resourceName)
	if err != nil {
		return 0, err
	}

	if !found {
		logger.Debug("resource-not-found")
		return 0, db.ResourceNotFoundError{Name: resourceName}
	}

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

	resourceTypes, err := scanner.dbPipeline.ResourceTypes()
	if err != nil {
		logger.Error("failed-to-get-resource-types", err)
		return 0, err
	}

	for _, parentType := range resourceTypes {
		if parentType.Name() != savedResource.Type() {
			continue
		}
		if parentType.Version() != nil {
			continue
		}

		err = scanner.typeScanner.Scan(logger.Session("resource-type-scanner"), parentType.Name())
		if err != nil {
			logger.Error("failed-to-scan-parent-resource-type-version", err)
			scanner.setResourceCheckError(logger, savedResource, err)
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

	source, err := creds.NewSource(scanner.variables, savedResource.Source()).Evaluate()
	if err != nil {
		logger.Error("failed-to-evaluate-resource-source", err)
		scanner.setResourceCheckError(logger, savedResource, err)
		return 0, err
	}

	resourceConfig, err := savedResource.SetResourceConfig(
		logger,
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
		_, found, err := resourceConfig.FindVersion(currentVersion)

		if err != nil {
			logger.Error("failed-to-find-pinned-version-on-resource", err, lager.Data{"pinned-version": currentVersion})
			chkErr := resourceConfig.SetCheckError(err)
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
				"resource_name":   resourceName,
				"resource_config": resourceConfig.ID(),
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
		savedResource,
		resourceConfig,
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
	resourceConfig db.ResourceConfig,
	fromVersion atc.Version,
	resourceTypes creds.VersionedResourceTypes,
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
		Tags:   savedResource.Tags(),
		TeamID: scanner.dbPipeline.TeamID(),
		Env:    metadata.Env(),
	}

	workerSpec := worker.WorkerSpec{
		ResourceType:  savedResource.Type(),
		Tags:          savedResource.Tags(),
		ResourceTypes: resourceTypes,
	}

	res, err := scanner.resourceFactory.NewResource(
		context.Background(),
		logger,
		db.NewResourceConfigCheckSessionContainerOwner(resourceConfig, ContainerExpiries),
		db.ContainerMetadata{
			Type: db.ContainerTypeCheck,
		},
		containerSpec,
		workerSpec,
		resourceTypes,
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

	logger.Debug("checking", lager.Data{
		"from": fromVersion,
	})

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	newVersions, err := res.Check(ctx, source, fromVersion)
	if err == context.DeadlineExceeded {
		err = fmt.Errorf("Timed out after %v while checking for new versions - perhaps increase your resource check timeout?", timeout)
	}

	resourceConfig.SetCheckError(err)
	metric.ResourceCheck{
		PipelineName: scanner.dbPipeline.Name(),
		ResourceName: savedResource.Name(),
		TeamName:     scanner.dbPipeline.TeamName(),
		Success:      err == nil,
	}.Emit(logger)

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
	}

	return nil
}

func swallowErrResourceScriptFailed(err error) error {
	if _, ok := err.(resource.ErrResourceScriptFailed); ok {
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
	setErr := savedResource.SetCheckError(err)
	if setErr != nil {
		logger.Error("failed-to-set-check-error", err)
	}
}

var errPipelineRemoved = errors.New("pipeline removed")
