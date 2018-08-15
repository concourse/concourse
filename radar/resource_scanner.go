package radar

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/metric"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

var GlobalResourceCheckTimeout time.Duration

type resourceScanner struct {
	clock                             clock.Clock
	resourceFactory                   resource.ResourceFactory
	resourceConfigCheckSessionFactory db.ResourceConfigCheckSessionFactory
	defaultInterval                   time.Duration
	dbPipeline                        db.Pipeline
	externalURL                       string
	variables                         creds.Variables
	typeScanner                       Scanner
}

func NewResourceScanner(
	clock clock.Clock,
	resourceFactory resource.ResourceFactory,
	resourceConfigCheckSessionFactory db.ResourceConfigCheckSessionFactory,
	defaultInterval time.Duration,
	dbPipeline db.Pipeline,
	externalURL string,
	variables creds.Variables,
	typeScanner Scanner,
) Scanner {
	return &resourceScanner{
		clock:                             clock,
		resourceFactory:                   resourceFactory,
		resourceConfigCheckSessionFactory: resourceConfigCheckSessionFactory,
		defaultInterval:                   defaultInterval,
		dbPipeline:                        dbPipeline,
		externalURL:                       externalURL,
		variables:                         variables,
		typeScanner:                       typeScanner,
	}
}

var ErrFailedToAcquireLock = errors.New("failed-to-acquire-lock")

func (scanner *resourceScanner) Run(logger lager.Logger, resourceName string) (time.Duration, error) {
	interval, err := scanner.scan(logger.Session("tick"), resourceName, nil, false)

	err = swallowErrResourceScriptFailed(err)

	return interval, err
}

func (scanner *resourceScanner) ScanFromVersion(logger lager.Logger, resourceName string, fromVersion atc.Version) error {
	_, err := scanner.scan(logger, resourceName, fromVersion, true)

	return err
}

func (scanner *resourceScanner) Scan(logger lager.Logger, resourceName string) error {
	_, err := scanner.scan(logger, resourceName, nil, true)

	err = swallowErrResourceScriptFailed(err)

	return err
}

func (scanner *resourceScanner) scan(logger lager.Logger, resourceName string, fromVersion atc.Version, mustComplete bool) (time.Duration, error) {
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

	interval, err := scanner.checkInterval(savedResource.CheckEvery())
	if err != nil {
		scanner.setResourceCheckError(logger, savedResource, err)

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

	resourceConfigCheckSession, err := scanner.resourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSession(
		logger,
		savedResource.Type(),
		source,
		versionedResourceTypes,
		ContainerExpiries,
	)
	if err != nil {
		logger.Error("failed-to-find-or-create-resource-config-check-session", err)
		scanner.setResourceCheckError(logger, savedResource, err)
		return 0, err
	}

	err = savedResource.SetResourceConfig(resourceConfigCheckSession.ResourceConfig().ID())
	if err != nil {
		logger.Error("failed-to-set-resource-config-id-on-resource", err)
		scanner.setResourceCheckError(logger, savedResource, err)
		return 0, err
	}

	for breaker := true; breaker == true; breaker = mustComplete {
		lock, acquired, err := scanner.dbPipeline.AcquireResourceCheckingLockWithIntervalCheck(
			logger,
			savedResource.Name(),
			resourceConfigCheckSession.ResourceConfig(),
			interval,
			mustComplete,
		)
		if err != nil {
			lockLogger.Error("failed-to-get-lock", err, lager.Data{
				"resource": resourceName,
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
		vr, _, err := scanner.dbPipeline.GetLatestVersionedResource(resourceName)
		if err != nil {
			logger.Error("failed-to-get-current-version", err)
			return interval, err
		}
		fromVersion = atc.Version(vr.Version)
	}

	return interval, scanner.check(
		logger,
		savedResource,
		resourceConfigCheckSession,
		fromVersion,
		versionedResourceTypes,
		source,
	)
}

func (scanner *resourceScanner) check(
	logger lager.Logger,
	savedResource db.Resource,
	resourceConfigCheckSession db.ResourceConfigCheckSession,
	fromVersion atc.Version,
	resourceTypes creds.VersionedResourceTypes,
	source atc.Source,
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
		},
		Tags:   savedResource.Tags(),
		TeamID: scanner.dbPipeline.TeamID(),
		Env:    metadata.Env(),
	}

	res, err := scanner.resourceFactory.NewResource(
		context.Background(),
		logger,
		db.NewResourceConfigCheckSessionContainerOwner(resourceConfigCheckSession, scanner.dbPipeline.TeamID()),
		db.ContainerMetadata{
			Type: db.ContainerTypeCheck,
		},
		containerSpec,
		resourceTypes,
		worker.NoopImageFetchingDelegate{},
	)

	if err != nil {
		logger.Error("failed-to-initialize-new-container", err)
		scanner.setResourceCheckError(logger, savedResource, err)
		return err
	}

	logger.Debug("checking", lager.Data{
		"from": fromVersion,
	})

	timeout, err := scanner.parseResourceCheckTimeoutOrDefault(savedResource.CheckTimeout())
	if err != nil {
		scanner.setResourceCheckError(logger, savedResource, err)
		logger.Error("failed-to-read-check-timeout", err)
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	newVersions, err := res.Check(ctx, source, fromVersion)
	if err == context.DeadlineExceeded {
		err = fmt.Errorf("Timed out after %v while checking for new versions - perhaps increase your resource check timeout?", timeout)
	}

	scanner.setResourceCheckError(logger, savedResource, err)
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
	setErr := scanner.dbPipeline.SetResourceCheckError(savedResource, err)
	if setErr != nil {
		logger.Error("failed-to-set-check-error", err)
	}
}

var errPipelineRemoved = errors.New("pipeline removed")
