package gc

import (
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/metric"
	"github.com/concourse/atc/worker"
)

const HijackedContainerTimeout = 5 * time.Minute

//go:generate counterfeiter . containerFactory

type containerFactory interface {
	FindContainersForDeletion() ([]db.CreatingContainer, []db.CreatedContainer, []db.DestroyingContainer, error)
}

type containerCollector struct {
	rootLogger       lager.Logger
	containerFactory containerFactory
	jobRunner        WorkerJobRunner
}

func NewContainerCollector(
	logger lager.Logger,
	containerFactory containerFactory,
	jobRunner WorkerJobRunner,
) Collector {
	return &containerCollector{
		rootLogger:       logger,
		containerFactory: containerFactory,
		jobRunner:        jobRunner,
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

	creatingContainers, createdContainers, destroyingContainers, err := c.containerFactory.FindContainersForDeletion()
	if err != nil {
		logger.Error("failed-to-get-containers-for-deletion", err)
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

	logger.Debug("found-containers-for-deletion", lager.Data{
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

	for _, creatingContainer := range creatingContainers {
		cLog := logger.Session("mark-creating-as-created", lager.Data{
			"container": creatingContainer.Handle(),
		})

		createdContainer, err := creatingContainer.Created()
		if err != nil {
			cLog.Error("failed-to-transition", err)
			continue
		}

		createdContainers = append(createdContainers, createdContainer)
	}

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

		logger.Debug("destroyed-in-garden")

		metric.ContainersDeleted.Inc()
	}

	ok, err := container.Destroy()
	if err != nil {
		logger.Error("failed-to-destroy-database-container", err)
		return
	}

	if !ok {
		logger.Info("could-not-destroy-database-container")
		return
	}

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
