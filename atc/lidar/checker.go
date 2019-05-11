package lidar

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/worker"
)

var ErrFailedToAcquireLock = errors.New("failed to acquire lock")

func NewChecker(
	logger lager.Logger,
	checkFactory db.CheckFactory,
	resourceFactory resource.ResourceFactory,
	secrets creds.Secrets,
	pool worker.Pool,
	externalURL string,
) *checker {
	return &checker{
		logger,
		checkFactory,
		resourceFactory,
		secrets,
		pool,
		externalURL,
	}
}

type checker struct {
	logger          lager.Logger
	checkFactory    db.CheckFactory
	resourceFactory resource.ResourceFactory
	secrets         creds.Secrets
	pool            worker.Pool
	externalURL     string
}

func (c *checker) Run(ctx context.Context) error {

	checks, err := c.checkFactory.Checks()
	if err != nil {
		c.logger.Error("failed-to-fetch-resource-checks", err)
		return err
	}

	waitGroup := new(sync.WaitGroup)

	for _, check := range checks {
		waitGroup.Add(1)
		go c.check(ctx, check, waitGroup)
	}

	waitGroup.Wait()

	return nil
}

func (c *checker) check(ctx context.Context, check db.Check, waitGroup *sync.WaitGroup) error {
	defer waitGroup.Done()

	if err := c.tryCheck(ctx, check); err != nil {
		if err == ErrFailedToAcquireLock {
			return err
		}

		if err = check.FinishWithError(err.Error()); err != nil {
			c.logger.Error("failed-to-update-resource-check-error", err)
			return err
		}
	}

	return nil
}

func (c *checker) tryCheck(ctx context.Context, check db.Check) error {

	resourceConfigScope, err := check.ResourceConfigScope()
	if err != nil {
		c.logger.Error("failed-to-fetch-resource-config-scope", err)
		return err
	}

	resource, err := resourceConfigScope.AnyResource()
	if err != nil {
		c.logger.Error("failed-to-fetch-resource", err)
		return err
	}

	logger := c.logger.Session("check", lager.Data{
		"resource_id":        resource.ID(),
		"resource_name":      resource.Name(),
		"resource_config_id": resourceConfigScope.ResourceConfig().ID(),
	})

	lock, acquired, err := resourceConfigScope.AcquireCheckingLock(logger)
	if err != nil {
		logger.Error("failed-to-get-lock", err)
		return ErrFailedToAcquireLock
	}

	if !acquired {
		logger.Debug("lock-not-acquired")
		return ErrFailedToAcquireLock
	}

	defer lock.Release()

	if err = check.Start(); err != nil {
		logger.Error("failed-to-start-resource-check", err)
		return err
	}

	parent, err := resource.ParentResourceType()
	if err != nil {
		logger.Error("failed-to-fetch-parent-type", err)
		return err
	}

	if parent.Version() == nil {
		err = errors.New("parent resource has no version")
		logger.Error("failed-due-to-missing-parent-version", err)
		return err
	}

	checkable, err := c.createCheckable(logger, ctx, resource, resourceConfigScope.ResourceConfig(), versionedResourceTypes)
	if err != nil {
		logger.Error("failed-to-create-resource-checkable", err)
		return err
	}

	deadline, cancel := context.WithTimeout(ctx, check.Timeout())
	defer cancel()

	logger.Debug("checking", lager.Data{"from": check.FromVersion()})

	versions, err := checkable.Check(deadline, source, check.FromVersion())
	if err != nil {
		if err == context.DeadlineExceeded {
			return fmt.Errorf("Timed out after %v while checking for new versions", check.Timeout())
		}
		return err
	}

	if err = resourceConfigScope.SaveVersions(versions); err != nil {
		logger.Error("failed-to-save-versions", err)
		return err
	}

	return check.Finish()
}

func (c *checker) createCheckable(
	logger lager.Logger,
	ctx context.Context,
	dbResource db.Resource,
	dbResourceConfig db.ResourceConfig,
	versionedResourceTypes creds.VersionedResourceTypes,
) (resource.Resource, error) {

	metadata := resource.TrackerMetadata{
		ResourceName: dbResource.Name(),
		PipelineName: dbResource.PipelineName(),
		ExternalURL:  c.externalURL,
	}

	containerSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: dbResource.Type(),
		},
		BindMounts: []worker.BindMountSource{
			&worker.CertsVolumeMount{Logger: logger},
		},
		Tags:   dbResource.Tags(),
		TeamID: dbResource.TeamID(),
		Env:    metadata.Env(),
	}

	workerSpec := worker.WorkerSpec{
		ResourceType:  dbResource.Type(),
		Tags:          dbResource.Tags(),
		ResourceTypes: versionedResourceTypes,
		TeamID:        dbResource.TeamID(),
	}

	owner := db.NewResourceConfigCheckSessionContainerOwner(
		step.metadata.ResourceConfigID,
		step.metadata.BaseResourceTypeID,
		db.ContainerOwnerExpiries{
			GraceTime: 2 * time.Minute,
			Min:       5 * time.Minute,
			Max:       1 * time.Hour,
		},
	)

	containerMetadata := db.ContainerMetadata{
		Type: db.ContainerTypeCheck,
	}

	chosenWorker, err := c.pool.FindOrChooseWorkerForContainer(
		logger,
		owner,
		containerSpec,
		workerSpec,
		worker.NewRandomPlacementStrategy(),
	)
	if err != nil {
		return nil, err
	}

	container, err := chosenWorker.FindOrCreateContainer(
		ctx,
		logger,
		worker.NoopImageFetchingDelegate{},
		owner,
		containerMetadata,
		containerSpec,
		versionedResourceTypes,
	)
	if err != nil {
		return nil, err
	}

	return c.resourceFactory.NewResourceForContainer(container), nil
}
