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
	MarkContainersForDeletion() error
	FindContainersMarkedForDeletion() ([]dbng.DestroyingContainer, error)
	FindHijackedContainersForDeletion() ([]dbng.CreatedContainer, error)
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

type GardenClientFactory func(*dbng.Worker) (garden.Client, error)

func NewGardenClientFactory() GardenClientFactory {
	return func(w *dbng.Worker) (garden.Client, error) {
		if w.GardenAddr == nil {
			return nil, errors.New("worker does not have a garden address")
		}

		gconn := connection.New("tcp", *w.GardenAddr)
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

	workersByName := map[string]*dbng.Worker{}
	for _, w := range workers {
		workersByName[w.Name] = w
	}

	hijackedContainersForDeletion, err := c.containerFactory.FindHijackedContainersForDeletion()
	if err != nil {
		logger.Error("failed-to-get-hijacked-containers-for-deletion", err)
		return err
	}

	for _, hijackedContainer := range hijackedContainersForDeletion {
		cLog := logger.Session("mark-hijacked-container", lager.Data{
			"container": hijackedContainer.Handle(),
			"worker":    hijackedContainer.WorkerName(),
		})

		c.markHijackedContainerAsDestroying(cLog, hijackedContainer, workersByName)
	}

	err = c.containerFactory.MarkContainersForDeletion()
	if err != nil {
		logger.Error("marking-build-containers-for-deletion", err)
	}

	containersToDelete, err := c.findContainersToDelete(logger)
	if err != nil {
		return err
	}

	for _, container := range containersToDelete {
		cLog := logger.Session("destroy-container", lager.Data{
			"container": container.Handle(),
			"worker":    container.WorkerName(),
		})

		c.tryToDestroyContainer(cLog, container, workersByName)
	}

	return nil
}

func (c *containerCollector) markHijackedContainerAsDestroying(
	logger lager.Logger,
	hijackedContainer dbng.CreatedContainer,
	workersByName map[string]*dbng.Worker,
) {
	w, found := workersByName[hijackedContainer.WorkerName()]
	if !found {
		logger.Info("worker-not-found")
		return
	}

	gclient, err := c.gardenClientFactory(w)
	if err != nil {
		logger.Error("failed-to-get-garden-client-for-worker", err)
		return
	}

	gardenContainer, err := gclient.Lookup(hijackedContainer.Handle())
	if err != nil {
		if _, ok := err.(garden.ContainerNotFoundError); ok {
			logger.Debug("hijacked-container-not-found-in-garden")

			_, err = hijackedContainer.Destroying()
			if err != nil {
				logger.Error("failed-to-mark-container-as-destroying", err)
				return
			}
		}

		logger.Error("failed-to-lookup-garden-container", err)
		return
	} else {
		err = gardenContainer.SetGraceTime(HijackedContainerTimeout)
		if err != nil {
			logger.Error("failed-to-set-grace-time-on-hijacked-container", err)
			return
		}

		_, err = hijackedContainer.Discontinue()
		if err != nil {
			logger.Error("failed-to-mark-container-as-destroying", err)
			return
		}
	}
}

func (c *containerCollector) findContainersToDelete(logger lager.Logger) ([]dbng.DestroyingContainer, error) {
	containers, err := c.containerFactory.FindContainersMarkedForDeletion()
	if err != nil {
		logger.Error("failed-to-find-containers-for-deletion", err)
		return nil, err
	}

	if len(containers) > 0 {
		containerHandles := []string{}
		for _, container := range containers {
			containerHandles = append(containerHandles, container.Handle())
		}

		logger.Debug("found-containers-for-deletion", lager.Data{
			"containers": containerHandles,
		})
	}

	return containers, nil
}

func (c *containerCollector) tryToDestroyContainer(logger lager.Logger, container dbng.DestroyingContainer, workersByName map[string]*dbng.Worker) {
	logger.Debug("start")
	defer logger.Debug("done")

	w, found := workersByName[container.WorkerName()]
	if !found {
		logger.Info("worker-not-found")
		return
	}

	gclient, err := c.gardenClientFactory(w)
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
