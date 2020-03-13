package gc

import (
	"context"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
	"github.com/hashicorp/go-multierror"
)

type containerCollector struct {
	containerRepository         db.ContainerRepository
	missingContainerGracePeriod time.Duration
	hijackContainerGracePeriod  time.Duration
}

func NewContainerCollector(
	containerRepository db.ContainerRepository,
	missingContainerGracePeriod time.Duration,
	hijackContainerGracePeriod time.Duration,
) *containerCollector {
	return &containerCollector{
		containerRepository:         containerRepository,
		missingContainerGracePeriod: missingContainerGracePeriod,
		hijackContainerGracePeriod:  hijackContainerGracePeriod,
	}
}

func (c *containerCollector) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("container-collector")

	logger.Debug("start")
	defer logger.Debug("done")

	start := time.Now()
	defer func() {
		metric.ContainerCollectorDuration{
			Duration: time.Since(start),
		}.Emit(logger)
	}()

	var errs error

	err := c.cleanupOrphanedContainers(logger.Session("orphaned-containers"))
	if err != nil {
		errs = multierror.Append(errs, err)
		logger.Error("failed-to-clean-up-orphaned-containers", err)
	}

	err = c.markFailedContainersAsDestroying(logger.Session("failed-containers"))
	if err != nil {
		errs = multierror.Append(errs, err)
		logger.Error("failed-to-clean-up-failed-containers", err)
	}

	_, err = c.containerRepository.RemoveMissingContainers(c.missingContainerGracePeriod)
	if err != nil {
		errs = multierror.Append(errs, err)
		logger.Error("failed-to-clean-up-missing-containers", err)
	}

	return errs
}

func (c *containerCollector) markFailedContainersAsDestroying(logger lager.Logger) error {

	numFailedContainers, err := c.containerRepository.DestroyFailedContainers()
	if err != nil {
		logger.Error("failed-to-find-failed-containers-for-deletion", err)
		return err
	}

	if numFailedContainers > 0 {
		logger.Debug("found-failed-containers-for-deletion", lager.Data{
			"number": numFailedContainers,
		})
	}

	metric.FailedContainersToBeGarbageCollected{
		Containers: numFailedContainers,
	}.Emit(logger)

	return nil
}

func (c *containerCollector) cleanupOrphanedContainers(logger lager.Logger) error {

	creatingContainers, createdContainers, destroyingContainers, err := c.containerRepository.FindOrphanedContainers()
	if err != nil {
		logger.Error("failed-to-get-orphaned-containers-for-deletion", err)
		return err
	}

	if len(creatingContainers) > 0 || len(createdContainers) > 0 || len(destroyingContainers) > 0 {
		logger.Debug("found-orphaned-containers-for-deletion", lager.Data{
			"creating-containers-num":   len(creatingContainers),
			"created-containers-num":    len(createdContainers),
			"destroying-containers-num": len(destroyingContainers),
		})
	}

	metric.CreatingContainersToBeGarbageCollected{
		Containers: len(creatingContainers),
	}.Emit(logger)

	metric.CreatedContainersToBeGarbageCollected{
		Containers: len(createdContainers),
	}.Emit(logger)

	metric.DestroyingContainersToBeGarbageCollected{
		Containers: len(destroyingContainers),
	}.Emit(logger)

	for _, createdContainer := range createdContainers {

		if time.Since(createdContainer.LastHijack()) > c.hijackContainerGracePeriod {
			_, err := createdContainer.Destroying()
			if err != nil {
				logger.Error("failed-to-transition", err, lager.Data{"container": createdContainer.Handle()})
				continue
			}
		} else {
			_, err = createdContainer.Discontinue()
			if err != nil {
				logger.Error("failed-to-discontinue-container", err)
				continue
			}
		}
	}

	return nil
}
