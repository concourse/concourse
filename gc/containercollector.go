package gc

import (
	"errors"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/metric"
	"github.com/concourse/atc/worker"
)

const HijackedContainerTimeout = 5 * time.Minute

var containerCollectorFailedErr = errors.New("container collector failed")

type containerCollector struct {
	rootLogger          lager.Logger
	containerRepository db.ContainerRepository
	jobRunner           WorkerJobRunner
}

func NewContainerCollector(
	logger lager.Logger,
	containerRepository db.ContainerRepository,
	jobRunner WorkerJobRunner,
) Collector {
	return &containerCollector{
		rootLogger:          logger,
		containerRepository: containerRepository,
		jobRunner:           jobRunner,
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

func (c *containerCollector) Run() error {
	logger := c.rootLogger.Session("run")

	logger.Debug("start")
	defer logger.Debug("done")

	var err error

	orphanedErr := c.cleanupOrphanedContainers(logger.Session("orphaned-containers"))
	if orphanedErr != nil {
		c.rootLogger.Error("container-collector", orphanedErr)
		err = containerCollectorFailedErr
	}

	failedErr := c.cleanupFailedContainers(logger.Session("failed-containers"))
	if failedErr != nil {
		c.rootLogger.Error("container-collector", failedErr)
		err = containerCollectorFailedErr
	}

	return err
}

func (c *containerCollector) cleanupFailedContainers(logger lager.Logger) error {

	failedContainers, err := c.containerRepository.FindFailedContainers()
	if err != nil {
		logger.Error("failed-to-find-failed-containers-for-deletion", err)
		return err
	}

	failedContainerHandles := []string{}

	if len(failedContainers) > 0 {
		for _, container := range failedContainers {
			failedContainerHandles = append(failedContainerHandles, container.Handle())
		}
	}

	logger.Debug("found-failed-containers-for-deletion", lager.Data{
		"failed-containers": failedContainerHandles,
	})

	metric.FailedContainersToBeGarbageCollected{
		Containers: len(failedContainerHandles),
	}.Emit(logger)

	for _, failedContainer := range failedContainers {
		// prevent closure from capturing last value of loop
		container := failedContainer

		destroyDBContainer(logger, container)
	}

	return nil
}

func (c *containerCollector) cleanupOrphanedContainers(logger lager.Logger) error {
	creatingContainers, createdContainers, destroyingContainers, err := c.containerRepository.FindOrphanedContainers()

	if err != nil {
		logger.Error("failed-to-get-orphaned-containers-for-deletion", err)
		return err
	}

	creatingContainerHandles := []string{}
	createdContainerHandles := []string{}
	destroyingContainerHandles := []string{}

	if len(creatingContainers) > 0 {
		for _, container := range creatingContainers {
			creatingContainerHandles = append(creatingContainerHandles, container.Handle())
		}
	}

	if len(createdContainers) > 0 {
		for _, container := range createdContainers {
			createdContainerHandles = append(createdContainerHandles, container.Handle())
		}
	}

	if len(destroyingContainers) > 0 {
		for _, container := range destroyingContainers {
			destroyingContainerHandles = append(destroyingContainerHandles, container.Handle())
		}
	}

	logger.Debug("found-orphaned-containers-for-deletion", lager.Data{
		"creating-containers":   creatingContainerHandles,
		"created-containers":    createdContainerHandles,
		"destroying-containers": destroyingContainerHandles,
	})

	metric.CreatingContainersToBeGarbageCollected{
		Containers: len(creatingContainerHandles),
	}.Emit(logger)

	metric.CreatedContainersToBeGarbageCollected{
		Containers: len(createdContainerHandles),
	}.Emit(logger)

	metric.DestroyingContainersToBeGarbageCollected{
		Containers: len(destroyingContainerHandles),
	}.Emit(logger)

	for _, createdContainer := range createdContainers {
		// prevent closure from capturing last value of loop
		container := createdContainer

		c.jobRunner.Try(logger,
			container.WorkerName(),
			&job{
				JobName: container.Handle(),
				RunFunc: destroyCreatedContainer(logger, container),
			},
		)
	}

	for _, destroyingContainer := range destroyingContainers {
		// prevent closure from capturing last value of loop
		container := destroyingContainer

		c.jobRunner.Try(logger,
			container.WorkerName(),
			&job{
				JobName: container.Handle(),
				RunFunc: destroyDestroyingContainer(logger, container),
			},
		)
	}

	return nil
}

func destroyCreatedContainer(logger lager.Logger, container db.CreatedContainer) func(worker.Worker) {
	return func(workerClient worker.Worker) {
		var destroyingContainer db.DestroyingContainer
		var cLog lager.Logger

		if container.IsHijacked() {
			cLog = logger.Session("mark-hijacked-container", lager.Data{
				"container": container.Handle(),
				"worker":    workerClient.Name(),
			})

			var err error
			destroyingContainer, err = markHijackedContainerAsDestroying(cLog, container, workerClient.GardenClient())
			if err != nil {
				cLog.Error("failed-to-transition", err)
				return
			}
		} else {
			cLog = logger.Session("mark-created-as-destroying", lager.Data{
				"container": container.Handle(),
				"worker":    workerClient.Name(),
			})

			var err error
			destroyingContainer, err = container.Destroying()
			if err != nil {
				cLog.Error("failed-to-transition", err)
				return
			}
		}

		if destroyingContainer != nil {
			tryToDestroyContainer(cLog, destroyingContainer, workerClient)
		}
	}
}

func destroyDestroyingContainer(logger lager.Logger, container db.DestroyingContainer) func(worker.Worker) {
	return func(workerClient worker.Worker) {
		cLog := logger.Session("destroy-container", lager.Data{
			"container": container.Handle(),
			"worker":    workerClient.Name(),
		})

		tryToDestroyContainer(cLog, container, workerClient)
	}
}

func markHijackedContainerAsDestroying(
	logger lager.Logger,
	hijackedContainer db.CreatedContainer,
	gardenClient garden.Client,
) (db.DestroyingContainer, error) {
	gardenContainer, found, err := findContainer(gardenClient, hijackedContainer.Handle())
	if err != nil {
		logger.Error("failed-to-lookup-container-in-garden", err)
		return nil, err
	}

	if !found {
		logger.Debug("hijacked-container-not-found-in-garden")

		destroyingContainer, err := hijackedContainer.Destroying()
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

func tryToDestroyContainer(
	logger lager.Logger,
	container db.DestroyingContainer,
	workerClient worker.Worker,
) {
	logger.Debug("start")
	defer logger.Debug("done")

	gardenClient := workerClient.GardenClient()

	if container.IsDiscontinued() {
		logger.Debug("discontinued")

		_, found, err := findContainer(gardenClient, container.Handle())
		if err != nil {
			logger.Error("failed-to-lookup-container-in-garden", err)
			return
		}

		if found {
			logger.Debug("still-present-in-garden")
			return
		} else {
			logger.Debug("container-no-longer-present-in-garden")
		}
	} else {
		err := gardenClient.Destroy(container.Handle())
		if err != nil {
			if _, ok := err.(garden.ContainerNotFoundError); ok {
				logger.Debug("container-no-longer-present-in-garden")
			} else {
				logger.Error("failed-to-destroy-garden-container", err)
				return
			}
		}

	}

	logger.Debug("destroyed-in-garden")
	destroyDBContainer(logger, container)
}

type destroyableContainer interface {
	Destroy() (bool, error)
}

func destroyDBContainer(logger lager.Logger, dbContainer destroyableContainer) {
	logger.Debug("destroying")

	destroyed, err := dbContainer.Destroy()
	if err != nil {
		logger.Error("failed-to-destroy-database-container", err)
		return
	}

	if !destroyed {
		logger.Info("could-not-destroy-database-container")
		return
	}

	metric.ContainersDeleted.Inc()
	logger.Debug("destroyed-in-db")
}

func findContainer(gardenClient garden.Client, handle string) (garden.Container, bool, error) {
	gardenContainer, err := gardenClient.Lookup(handle)
	if err != nil {
		if _, ok := err.(garden.ContainerNotFoundError); ok {
			return nil, false, nil
		} else {
			return nil, false, err
		}
	}

	return gardenContainer, true, nil
}
