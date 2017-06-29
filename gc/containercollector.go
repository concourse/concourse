package gc

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
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
	workerPool       *WorkerPool
}

func NewContainerCollector(
	logger lager.Logger,
	containerFactory containerFactory,
	workerPool *WorkerPool,
) Collector {
	return &containerCollector{
		rootLogger:       logger,
		containerFactory: containerFactory,
		workerPool:       workerPool,
	}
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

		c.workerPool.Queue(logger, container.WorkerName(), JobFunc(func(workerClient worker.Worker) {
			var destroyingContainer db.DestroyingContainer
			var cLog lager.Logger

			if container.IsHijacked() {
				cLog = logger.Session("mark-hijacked-container", lager.Data{
					"container": container.Handle(),
					"worker":    workerClient.Name(),
				})

				var err error
				destroyingContainer, err = c.markHijackedContainerAsDestroying(cLog, container, workerClient)
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
				c.tryToDestroyContainer(cLog, destroyingContainer, workerClient)
			}
		}))
	}

	for _, destroyingContainer := range destroyingContainers {
		// prevent closure from capturing last value of loop
		container := destroyingContainer

		c.workerPool.Queue(logger, container.WorkerName(), JobFunc(func(workerClient worker.Worker) {
			cLog := logger.Session("destroy-container", lager.Data{
				"container": container.Handle(),
				"worker":    workerClient.Name(),
			})

			c.tryToDestroyContainer(cLog, container, workerClient)
		}))
	}

	return nil
}

func (c *containerCollector) markHijackedContainerAsDestroying(
	logger lager.Logger,
	hijackedContainer db.CreatedContainer,
	workerClient worker.Client,
) (db.DestroyingContainer, error) {
	gardenContainer, found, err := workerClient.FindContainerByHandle(logger, hijackedContainer.TeamID(), hijackedContainer.Handle())
	if err != nil {
		logger.Error("failed-to-lookup-garden-container", err)
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

func (c *containerCollector) tryToDestroyContainer(logger lager.Logger, container db.DestroyingContainer, workerClient worker.Client) {
	logger.Debug("start")
	defer logger.Debug("done")

	gardenContainer, found, err := workerClient.FindContainerByHandle(logger, container.TeamID(), container.Handle())
	if err != nil {
		logger.Error("failed-to-lookup-container-in-garden", err)
		return
	}

	if !found {
		logger.Debug("container-no-longer-present-in-garden")
	} else {
		if container.IsDiscontinued() {
			logger.Debug("waiting-for-garden-to-reap-it")
			return
		} else {
			err := gardenContainer.Destroy()
			if err != nil {
				logger.Error("failed-to-destroy-garden-container", err)
				return
			}

			logger.Debug("destroyed-in-garden")
		}
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
