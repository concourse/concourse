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
	logger              lager.Logger
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
		logger:              logger,
		containerFactory:    containerFactory,
		workerProvider:      workerProvider,
		gardenClientFactory: gardenClientFactory,
	}
}

type GardenClientFactory func(*dbng.Worker) (garden.Client, error)

func NewGardenClientFactory() GardenClientFactory {
	return func(w *dbng.Worker) (garden.Client, error) {
		if w.GardenAddr == nil {
			return nil, errors.New("worker-does-not-have-garden-address")
		}

		gconn := connection.New("tcp", *w.GardenAddr)
		return client.New(gconn), nil
	}
}

func (c *containerCollector) Run() error {
	workers, err := c.workerProvider.Workers()
	if err != nil {
		c.logger.Error("failed-to-get-workers", err)
		return err
	}
	workersByName := map[string]*dbng.Worker{}
	for _, w := range workers {
		workersByName[w.Name] = w
	}

	err = c.markHijackedContainersAsDestroying(workersByName)
	if err != nil {
		return err
	}

	err = c.containerFactory.MarkContainersForDeletion()
	if err != nil {
		c.logger.Error("marking-build-containers-for-deletion", err)
	}

	containersToDelete, err := c.findContainersToDelete()
	if err != nil {
		return err
	}

	for _, container := range containersToDelete {
		c.tryToDestroyContainer(container, workersByName)
	}

	c.logger.Debug("completed-deleting-containers")

	return nil
}

func (c *containerCollector) markHijackedContainersAsDestroying(workersByName map[string]*dbng.Worker) error {
	hijackedContainersForDeletion, err := c.containerFactory.FindHijackedContainersForDeletion()
	if err != nil {
		c.logger.Error("failed-to-get-hijacked-containers-for-deletion", err)
		return err
	}

	for _, hijackedContainer := range hijackedContainersForDeletion {
		w, found := workersByName[hijackedContainer.WorkerName()]
		if !found {
			c.logger.Info("worker-not-found", lager.Data{
				"worker-name": hijackedContainer.WorkerName(),
			})
			continue
		}

		gclient, err := c.gardenClientFactory(w)
		if err != nil {
			c.logger.Error("failed-to-get-garden-client-for-worker", err, lager.Data{
				"worker-name": hijackedContainer.WorkerName(),
			})
			continue
		}

		gardenContainer, err := gclient.Lookup(hijackedContainer.Handle())
		if err != nil {
			if _, ok := err.(garden.ContainerNotFoundError); ok {
				c.logger.Debug("hijacked-container-not-found-in-garden", lager.Data{
					"worker-name": hijackedContainer.WorkerName(),
					"handle":      hijackedContainer.Handle(),
				})

				_, err = hijackedContainer.Destroying()
				if err != nil {
					c.logger.Error("failed-to-mark-container-as-destroying", err, lager.Data{
						"worker-name": hijackedContainer.WorkerName(),
						"handle":      hijackedContainer.Handle(),
					})
					continue
				}
			}

			c.logger.Error("failed-to-lookup-garden-container", err, lager.Data{
				"worker-name": hijackedContainer.WorkerName(),
				"handle":      hijackedContainer.Handle(),
			})
			continue
		} else {
			err = gardenContainer.SetGraceTime(HijackedContainerTimeout)
			if err != nil {
				c.logger.Error("failed-to-set-grace-time-on-hijacked-container", err, lager.Data{
					"worker-name": hijackedContainer.WorkerName(),
					"handle":      hijackedContainer.Handle(),
				})
				continue
			}

			_, err = hijackedContainer.Discontinue()
			if err != nil {
				c.logger.Error("failed-to-mark-container-as-destroying", err, lager.Data{
					"worker-name": hijackedContainer.WorkerName(),
					"handle":      hijackedContainer.Handle(),
				})
				continue
			}
		}
	}

	return nil
}

func (c *containerCollector) findContainersToDelete() ([]dbng.DestroyingContainer, error) {
	containers, err := c.containerFactory.FindContainersMarkedForDeletion()
	if err != nil {
		c.logger.Error("find-build-containers-for-deletion", err)
		return nil, err
	}
	containerHandles := []string{}
	for _, container := range containers {
		containerHandles = append(containerHandles, container.Handle())
	}
	c.logger.Debug("found-build-containers-for-deletion", lager.Data{
		"containers": containerHandles,
	})

	return containers, nil
}

func (c *containerCollector) tryToDestroyContainer(container dbng.DestroyingContainer, workersByName map[string]*dbng.Worker) {
	w, found := workersByName[container.WorkerName()]
	if !found {
		c.logger.Info("worker-not-found", lager.Data{
			"worker-name": container.WorkerName(),
		})
		return
	}

	gclient, err := c.gardenClientFactory(w)
	if err != nil {
		c.logger.Error("failed-to-get-garden-client-for-worker", err, lager.Data{
			"worker-name": container.WorkerName(),
		})
		return
	}

	if container.IsDiscontinued() {
		_, err := gclient.Lookup(container.Handle())
		if err != nil {
			if _, ok := err.(garden.ContainerNotFoundError); ok {
				c.logger.Debug("discontinued-container-no-longer-present-in-garden", lager.Data{
					"handle": container.Handle(),
				})

			} else {
				c.logger.Error("failed-to-lookup-container-in-garden", err, lager.Data{
					"worker-name": container.WorkerName(),
				})

				return
			}
		} else {
			c.logger.Debug("discontinued-container-still-present-in-garden", lager.Data{
				"handle": container.Handle(),
			})

			return
		}
	} else {
		err = gclient.Destroy(container.Handle())
		if err != nil {
			c.logger.Error("failed-to-destroy-garden-container", err, lager.Data{
				"worker-name": container.WorkerName(),
				"handle":      container.Handle(),
			})
			return
		}
	}

	ok, err := container.Destroy()
	if err != nil {
		c.logger.Error("failed-to-destroy-database-container", err, lager.Data{
			"handle": container.Handle(),
		})
		return
	}

	if !ok {
		c.logger.Info("container-provider-container-not-found", lager.Data{
			"handle": container.Handle(),
		})
		return
	}

	c.logger.Debug("completed-deleting-container", lager.Data{
		"handle": container.Handle(),
	})
}
