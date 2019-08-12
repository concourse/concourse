package gc

import (
	"context"
	"fmt"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/gclient"
	"github.com/hashicorp/go-multierror"
)

const HijackedContainerTimeout = 5 * time.Minute

type containerCollector struct {
	containerRepository         db.ContainerRepository
	jobRunner                   WorkerJobRunner
	missingContainerGracePeriod time.Duration
}

func NewContainerCollector(
	containerRepository db.ContainerRepository,
	jobRunner WorkerJobRunner,
	missingContainerGracePeriod time.Duration,
) Collector {
	return &containerCollector{
		containerRepository:         containerRepository,
		jobRunner:                   jobRunner,
		missingContainerGracePeriod: missingContainerGracePeriod,
	}
}

type job struct {
	JobName string
	RunFunc func(worker.Worker)
}

func (j *job) Name() string {
	return j.JobName
}

func (j *job) Run(w worker.Worker) {
	j.RunFunc(w)
}

func (c *containerCollector) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("container-collector")

	logger.Debug("start")
	defer logger.Debug("done")

	var errs error

	err := c.cleanupOrphanedContainers(logger.Session("orphaned-containers"))
	if err != nil {
		errs = multierror.Append(errs, err)
		logger.Error("failed-to-clean-up-orphaned-containers", err)
	}

	err = c.cleanupFailedContainers(logger.Session("failed-containers"))
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

func (c *containerCollector) cleanupFailedContainers(logger lager.Logger) error {
	failedContainersLen, err := c.containerRepository.DestroyFailedContainers()
	if err != nil {
		logger.Error("failed-to-find-failed-containers-for-deletion", err)
		return err
	}

	if failedContainersLen > 0 {
		logger.Debug("found-failed-containers-for-deletion", lager.Data{
			"number": failedContainersLen,
		})
	}

	metric.FailedContainersToBeGarbageCollected{
		Containers: failedContainersLen,
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

	var workerCreatedContainers = make(map[string][]db.CreatedContainer)

	for _, createdContainer := range createdContainers {
		containers, ok := workerCreatedContainers[createdContainer.WorkerName()]
		if ok {
			// update existing array
			containers = append(containers, createdContainer)
			workerCreatedContainers[createdContainer.WorkerName()] = containers
		} else {
			// create new array
			workerCreatedContainers[createdContainer.WorkerName()] = []db.CreatedContainer{createdContainer}
		}
	}

	logger.Debug("found-created-containers-for-deletion", lager.Data{
		"num-containers": len(createdContainers),
	})

	for worker, createdContainers := range workerCreatedContainers {
		go destroyNonHijackedCreatedContainers(logger, createdContainers)

		// prevent closure from capturing last value of loop
		c.jobRunner.Try(logger,
			worker,
			&job{
				JobName: fmt.Sprintf("destroy-hijacked-containers"),
				RunFunc: destroyHijackedCreatedContainers(logger, createdContainers),
			},
		)
	}

	return nil
}

func destroyNonHijackedCreatedContainers(logger lager.Logger, containers []db.CreatedContainer) {
	cLog := logger.Session("mark-created-as-destroying")

	for _, container := range containers {
		if container.IsHijacked() {
			continue
		}

		_, err := container.Destroying()
		if err != nil {
			cLog.Error("failed-to-transition", err, lager.Data{
				"container": container.Handle(),
			})
			return
		}
	}
}

func destroyHijackedCreatedContainers(logger lager.Logger, containers []db.CreatedContainer) func(worker.Worker) {
	return func(gardenWorker worker.Worker) {
		cLog := logger.Session("mark-hijacked-container", lager.Data{
			"worker": gardenWorker.Name(),
		})

		for _, container := range containers {
			if !container.IsHijacked() {
				continue
			}

			_, err := markHijackedContainerAsDestroying(cLog, container, gardenWorker.GardenClient())
			if err != nil {
				cLog.Error("failed-to-transition", err, lager.Data{
					"container": container.Handle(),
				})
				return
			}
		}
	}
}

func markHijackedContainerAsDestroying(
	logger lager.Logger,
	hijackedContainer db.CreatedContainer,
	gardenClient gclient.Client,
) (db.DestroyingContainer, error) {

	gardenContainer, found, err := findContainer(gardenClient, hijackedContainer.Handle())
	if err != nil {
		logger.Error("failed-to-lookup-container-in-garden", err)
		return nil, err
	}

	if !found {
		var destroyingContainer db.DestroyingContainer
		logger.Debug("hijacked-container-not-found-in-garden")

		destroyingContainer, err = hijackedContainer.Destroying()
		if err != nil {
			logger.Error("failed-to-mark-container-as-destroying", err)
			return nil, err
		}
		return destroyingContainer, nil
	}

	err = gardenContainer.SetGraceTime(HijackedContainerTimeout)
	if err != nil {
		logger.Error("failed-to-set-grace-time-on-hijacked-container", err)
		return nil, err
	}

	_, err = hijackedContainer.Discontinue()
	if err != nil {
		logger.Error("failed-to-mark-container-as-destroying", err)
		return nil, err
	}

	return nil, nil
}

func findContainer(gardenClient gclient.Client, handle string) (gclient.Container, bool, error) {
	gardenContainer, err := gardenClient.Lookup(handle)
	if err != nil {
		if _, ok := err.(garden.ContainerNotFoundError); ok {
			return nil, false, nil
		}
		return nil, false, err
	}
	return gardenContainer, true, nil
}
