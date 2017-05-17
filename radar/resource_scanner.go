package radar

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

type resourceScanner struct {
	clock           clock.Clock
	resourceFactory resource.ResourceFactory
	defaultInterval time.Duration
	dbPipeline      dbng.Pipeline
	externalURL     string
}

func NewResourceScanner(
	clock clock.Clock,
	resourceFactory resource.ResourceFactory,
	defaultInterval time.Duration,
	dbPipeline dbng.Pipeline,
	externalURL string,
) Scanner {
	return &resourceScanner{
		clock:           clock,
		resourceFactory: resourceFactory,
		defaultInterval: defaultInterval,
		dbPipeline:      dbPipeline,
		externalURL:     externalURL,
	}
}

type ResourceNotFoundError struct {
	Name string
}

func (e ResourceNotFoundError) Error() string {
	return fmt.Sprintf("resource '%s' not found", e.Name)
}

var ErrFailedToAcquireLock = errors.New("failed-to-acquire-lock")

func (scanner *resourceScanner) Run(logger lager.Logger, resourceName string) (time.Duration, error) {
	savedResource, found, err := scanner.dbPipeline.Resource(resourceName)
	if err != nil {
		return 0, err
	}

	if !found {
		return 0, ResourceNotFoundError{Name: resourceName}
	}

	interval, err := scanner.checkInterval(savedResource.CheckEvery())
	if err != nil {
		setErr := scanner.dbPipeline.SetResourceCheckError(savedResource, err)
		if setErr != nil {
			logger.Error("failed-to-set-check-error", err)
		}

		return 0, err
	}

	lockLogger := logger.Session("lock", lager.Data{
		"resource": resourceName,
	})

	lock, acquired, err := scanner.dbPipeline.AcquireResourceCheckingLockWithIntervalCheck(
		logger,
		savedResource,
		interval,
		false,
	)

	if err != nil {
		lockLogger.Error("failed-to-get-lock", err, lager.Data{
			"resource": resourceName,
		})
		return interval, ErrFailedToAcquireLock
	}

	if !acquired {
		lockLogger.Debug("did-not-get-lock")
		return interval, ErrFailedToAcquireLock
	}

	defer lock.Release()

	vr, _, err := scanner.dbPipeline.GetLatestVersionedResource(resourceName)
	if err != nil {
		logger.Error("failed-to-get-current-version", err)
		return interval, err
	}

	resourceTypes, err := scanner.dbPipeline.ResourceTypes()
	if err != nil {
		logger.Error("failed-to-get-resource-types", err)
		return 0, err
	}

	err = swallowErrResourceScriptFailed(
		scanner.scan(
			logger.Session("tick"),
			savedResource,
			atc.Version(vr.Version),
			resourceTypes.Deserialize(),
		),
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

	savedResource, found, err := scanner.dbPipeline.Resource(resourceName)
	if err != nil {
		return err
	}

	if !found {
		logger.Debug("resource-not-found")
		return dbng.ResourceNotFoundError{Name: resourceName}
	}

	interval, err := scanner.checkInterval(savedResource.CheckEvery())
	if err != nil {
		setErr := scanner.dbPipeline.SetResourceCheckError(savedResource, err)
		if setErr != nil {
			logger.Error("failed-to-set-check-error", err)
		}

		return err
	}

	for {
		lock, acquired, err := scanner.dbPipeline.AcquireResourceCheckingLockWithIntervalCheck(
			logger,
			savedResource,
			interval,
			true,
		)
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

	resourceTypes, err := scanner.dbPipeline.ResourceTypes()
	if err != nil {
		logger.Error("failed-to-get-resource-types", err)
		return err
	}

	versionedResourceTypes := resourceTypes.Deserialize()

	return scanner.scan(logger, savedResource, fromVersion, versionedResourceTypes)
}

func (scanner *resourceScanner) Scan(logger lager.Logger, resourceName string) error {
	vr, _, err := scanner.dbPipeline.GetLatestVersionedResource(resourceName)
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
	savedResource dbng.Resource,
	fromVersion atc.Version,
	resourceTypes atc.VersionedResourceTypes,
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

	if savedResource.Paused() {
		logger.Debug("resource-paused")
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
			Privileged:   true,
		},
		Tags:   savedResource.Tags(),
		TeamID: scanner.dbPipeline.TeamID(),
		Env:    metadata.Env(),
	}

	res, err := scanner.resourceFactory.NewCheckResource(
		logger,
		nil,
		dbng.ForResource(savedResource.ID()),
		savedResource.Type(),
		savedResource.Source(),
		dbng.ContainerMetadata{
			Type: dbng.ContainerTypeCheck,
		},
		containerSpec,
		resourceTypes,
		worker.NoopImageFetchingDelegate{},
	)
	if err != nil {
		logger.Error("failed-to-initialize-new-container", err)
		setErr := scanner.dbPipeline.SetResourceCheckError(savedResource, err)
		if setErr != nil {
			logger.Error("failed-to-set-check-error", err)
		}
		return err
	}

	logger.Debug("checking", lager.Data{
		"from": fromVersion,
	})

	newVersions, err := res.Check(savedResource.Source(), fromVersion)

	setErr := scanner.dbPipeline.SetResourceCheckError(savedResource, err)
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

	err = scanner.dbPipeline.SaveResourceVersions(atc.ResourceConfig{
		Name: savedResource.Name(),
		Type: savedResource.Type(),
	}, newVersions)
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

var errPipelineRemoved = errors.New("pipeline removed")
