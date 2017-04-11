package gcng

import (
	"errors"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/client"
	"code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/dbng"
)

const HijackedContainerTimeout = 5 * time.Minute

//go:generate counterfeiter . containerFactory

type containerFactory interface {
	FindContainersForDeletion() ([]dbng.CreatingContainer, []dbng.CreatedContainer, []dbng.DestroyingContainer, error)
}

type containerCollector struct {
	rootLogger          lager.Logger
	containerFactory    containerFactory
	workerProvider      dbng.WorkerFactory
	gardenClientFactory GardenClientFactory
}

func NewContainerCollector(
	logger lager.Logger,
	containerFactory containerFactory,
	workerProvider dbng.WorkerFactory,
	gardenClientFactory GardenClientFactory,
) Collector {
	return &containerCollector{
		rootLogger:          logger,
		containerFactory:    containerFactory,
		workerProvider:      workerProvider,
		gardenClientFactory: gardenClientFactory,
	}
}

type GardenClientFactory func(dbng.Worker, lager.Logger) (garden.Client, error)

func NewGardenClientFactory() GardenClientFactory {
	return func(w dbng.Worker, logger lager.Logger) (garden.Client, error) {
		if w.GardenAddr() == nil {
			return nil, errors.New("worker does not have a garden address")
		}

		gconn := connection.NewWithDialerAndLogger(keepaliveDialer(*w.GardenAddr()), logger)
		return client.New(gconn), nil
	}
}

func (c *containerCollector) Run() error {
	logger := c.rootLogger.Session("run")

	logger.Debug("start")
	defer logger.Debug("done")

	workers, err := c.workerProvider.Workers()
	if err != nil {
		logger.Error("failed-to-get-workers", err)
		return err
	}

	workersByName := map[string]dbng.Worker{}
	for _, w := range workers {
		workersByName[w.Name()] = w
	}

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
		if createdContainer.IsHijacked() {
			cLog := logger.Session("mark-hijacked-container", lager.Data{
				"container": createdContainer.Handle(),
				"worker":    createdContainer.WorkerName(),
			})

			destroyingContainer := c.markHijackedContainerAsDestroying(cLog, createdContainer, workersByName)
			if destroyingContainer != nil {
				destroyingContainers = append(destroyingContainers, destroyingContainer)
			}
		} else {
			cLog := logger.Session("mark-created-as-destroying", lager.Data{
				"container": createdContainer.Handle(),
				"worker":    createdContainer.WorkerName(),
			})

			destroyingContainer, err := createdContainer.Destroying()
			if err != nil {
				cLog.Error("failed-to-transition", err)
				continue
			}

			destroyingContainers = append(destroyingContainers, destroyingContainer)
		}
	}

	for _, destroyingContainer := range destroyingContainers {
		cLog := logger.Session("destroy-container", lager.Data{
			"container": destroyingContainer.Handle(),
			"worker":    destroyingContainer.WorkerName(),
		})

		c.tryToDestroyContainer(cLog, destroyingContainer, workersByName)
	}

	return nil
}

func (c *containerCollector) markHijackedContainerAsDestroying(
	logger lager.Logger,
	hijackedContainer dbng.CreatedContainer,
	workersByName map[string]dbng.Worker,
) dbng.DestroyingContainer {
	w, found := workersByName[hijackedContainer.WorkerName()]
	if !found {
		logger.Info("worker-not-found")
		return nil
	}

	gclient, err := c.gardenClientFactory(w, logger)
	if err != nil {
		logger.Error("failed-to-get-garden-client-for-worker", err)
		return nil
	}

	gardenContainer, err := gclient.Lookup(hijackedContainer.Handle())
	if err != nil {
		if _, ok := err.(garden.ContainerNotFoundError); ok {
			logger.Debug("hijacked-container-not-found-in-garden")

			destroyingContainer, err := hijackedContainer.Destroying()
			if err != nil {
				logger.Error("failed-to-mark-container-as-destroying", err)
				return destroyingContainer
			}
		}

		logger.Error("failed-to-lookup-garden-container", err)
		return nil
	} else {
		err = gardenContainer.SetGraceTime(HijackedContainerTimeout)
		if err != nil {
			logger.Error("failed-to-set-grace-time-on-hijacked-container", err)
			return nil
		}

		_, err = hijackedContainer.Discontinue()
		if err != nil {
			logger.Error("failed-to-mark-container-as-destroying", err)
			return nil
		}
	}

	return nil
}

func (c *containerCollector) tryToDestroyContainer(logger lager.Logger, container dbng.DestroyingContainer, workersByName map[string]dbng.Worker) {
	logger.Debug("start")
	defer logger.Debug("done")

	w, found := workersByName[container.WorkerName()]
	if !found {
		logger.Info("worker-not-found")
		return
	}
	if w.State() == dbng.WorkerStateStalled || w.State() == dbng.WorkerStateLanded {
		logger.Debug("worker-is-not-available", lager.Data{"state": string(w.State())})
		return
	}

	gclient, err := c.gardenClientFactory(w, logger)
	if err != nil {
		logger.Error("failed-to-get-garden-client-for-worker", err)
		return
	}

	if container.IsDiscontinued() {
		logger.Debug("discontinued")

		_, err := gclient.Lookup(container.Handle())
		if err != nil {
			if _, ok := err.(garden.ContainerNotFoundError); ok {
				logger.Debug("container-no-longer-present-in-garden")
			} else {
				logger.Error("failed-to-lookup-container-in-garden", err)
				return
			}
		} else {
			logger.Debug("still-present-in-garden")
			return
		}
	} else {
		err = gclient.Destroy(container.Handle())
		if err != nil {
			if _, ok := err.(garden.ContainerNotFoundError); ok {
				logger.Debug("container-no-longer-present-in-garden")
			} else {
				logger.Error("failed-to-destroy-garden-container", err)
				return
			}
		}

		logger.Debug("destroyed-in-garden")
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
