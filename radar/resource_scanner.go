package radar

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

type resourceScanner struct {
	clock                             clock.Clock
	resourceFactory                   resource.ResourceFactory
	resourceConfigCheckSessionFactory db.ResourceConfigCheckSessionFactory
	defaultInterval                   time.Duration
	dbPipeline                        db.Pipeline
	externalURL                       string
	variables                         creds.Variables
}

func NewResourceScanner(
	clock clock.Clock,
	resourceFactory resource.ResourceFactory,
	resourceConfigCheckSessionFactory db.ResourceConfigCheckSessionFactory,
	defaultInterval time.Duration,
	dbPipeline db.Pipeline,
	externalURL string,
	variables creds.Variables,
) Scanner {
	return &resourceScanner{
		clock:                             clock,
		resourceFactory:                   resourceFactory,
		resourceConfigCheckSessionFactory: resourceConfigCheckSessionFactory,
		defaultInterval:                   defaultInterval,
		dbPipeline:                        dbPipeline,
		externalURL:                       externalURL,
		variables:                         variables,
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
	lockLogger := logger.Session("lock", lager.Data{
		"resource": resourceName,
	})

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

	resourceTypes, err := scanner.dbPipeline.ResourceTypes()
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
		setErr := scanner.dbPipeline.SetResourceCheckError(savedResource, err)
		if setErr != nil {
			logger.Error("failed-to-set-check-error", err)
		}
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
		setErr := scanner.dbPipeline.SetResourceCheckError(savedResource, err)
		if setErr != nil {
			logger.Error("failed-to-set-check-error", err)
		}
		return 0, err
	}

	err = savedResource.SetResourceConfig(resourceConfigCheckSession.ResourceConfig().ID)
	if err != nil {
		logger.Error("failed-to-set-resource-config-id-on-resource", err)
		setErr := scanner.dbPipeline.SetResourceCheckError(savedResource, err)
		if setErr != nil {
			logger.Error("failed-to-set-check-error", err)
		}
		return 0, err
	}

	lock, acquired, err := scanner.dbPipeline.AcquireResourceCheckingLockWithIntervalCheck(
		logger,
		savedResource.Name(),
		resourceConfigCheckSession.ResourceConfig(),
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

	err = swallowErrResourceScriptFailed(
		scanner.scan(
			logger.Session("tick"),
			savedResource,
			resourceConfigCheckSession,
			atc.Version(vr.Version),
			versionedResourceTypes,
			source,
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
		return db.ResourceNotFoundError{Name: resourceName}
	}

	interval, err := scanner.checkInterval(savedResource.CheckEvery())
	if err != nil {
		setErr := scanner.dbPipeline.SetResourceCheckError(savedResource, err)
		if setErr != nil {
			logger.Error("failed-to-set-check-error", err)
		}

		return err
	}

	resourceTypes, err := scanner.dbPipeline.ResourceTypes()
	if err != nil {
		logger.Error("failed-to-get-resource-types", err)
		return err
	}

	versionedResourceTypes := creds.NewVersionedResourceTypes(
		scanner.variables,
		resourceTypes.Deserialize(),
	)

	source, err := creds.NewSource(scanner.variables, savedResource.Source()).Evaluate()
	if err != nil {
		logger.Error("failed-to-evaluate-resource-source", err)
		return err
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
		setErr := scanner.dbPipeline.SetResourceCheckError(savedResource, err)
		if setErr != nil {
			logger.Error("failed-to-set-check-error", err)
		}
		return err
	}

	err = savedResource.SetResourceConfig(resourceConfigCheckSession.ResourceConfig().ID)
	if err != nil {
		logger.Error("failed-to-set-resource-config-id-on-resource", err)
		setErr := scanner.dbPipeline.SetResourceCheckError(savedResource, err)
		if setErr != nil {
			logger.Error("failed-to-set-check-error", err)
		}
		return err
	}

	for {
		lock, acquired, err := scanner.dbPipeline.AcquireResourceCheckingLockWithIntervalCheck(
			logger,
			savedResource.Name(),
			resourceConfigCheckSession.ResourceConfig(),
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

	return scanner.scan(logger, savedResource, resourceConfigCheckSession, fromVersion, versionedResourceTypes, source)
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
		logger,
		nil,
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
		setErr := scanner.dbPipeline.SetResourceCheckError(savedResource, err)
		if setErr != nil {
			logger.Error("failed-to-set-check-error", err)
		}
		return err
	}

	logger.Debug("checking", lager.Data{
		"from": fromVersion,
	})

	newVersions, err := res.Check(source, fromVersion)

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
