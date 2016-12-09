package gcng

import (
	"errors"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/client"
	"code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/dbng"
)

//go:generate counterfeiter . ContainerProvider
type ContainerProvider interface {
	MarkBuildContainersForDeletion() error
	FindContainersMarkedForDeletion() ([]*dbng.DestroyingContainer, error)
	ContainerDestroy(*dbng.DestroyingContainer) (bool, error)
}

type ContainerCollector struct {
	Logger              lager.Logger
	ContainerProvider   ContainerProvider
	WorkerProvider      dbng.WorkerFactory
	GardenClientFactory GardenClientFactory
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

func (c *ContainerCollector) Run() error {
	err := c.ContainerProvider.MarkBuildContainersForDeletion()
	if err != nil {
		c.Logger.Error("marking-build-containers-for-deletion", err)
	}
	c.Logger.Debug("completed-marking-build-containers-for-deletion")

	cs, err := c.ContainerProvider.FindContainersMarkedForDeletion()
	if err != nil {
		c.Logger.Error("find-build-containers-for-deletion", err)
		return err
	}
	c.Logger.Debug("found-build-containers-for-deletion", lager.Data{
		"containers": cs,
	})

	workers, err := c.WorkerProvider.Workers()
	if err != nil {
		c.Logger.Error("failed-to-get-workers", err)
		return err
	}
	workersByName := map[string]*dbng.Worker{}
	for _, w := range workers {
		workersByName[w.Name] = w
	}

	for _, container := range cs {
		w, found := workersByName[container.WorkerName]
		if !found {
			c.Logger.Info("worker-not-found", lager.Data{
				"workername": container.WorkerName,
			})
			continue
		}

		gclient, err := c.GardenClientFactory(w)
		if err != nil {
			c.Logger.Error("get-garden-client-for-worker", err, lager.Data{
				"worker": w,
			})
			continue
		}

		err = gclient.Destroy(container.Handle)
		if err != nil {
			c.Logger.Error("destroying-garden-container", err, lager.Data{
				"worker": w,
				"handle": container.Handle,
			})
			continue
		}

		ok, err := c.ContainerProvider.ContainerDestroy(container)
		if err != nil {
			c.Logger.Error("container-provider-container-destroy", err, lager.Data{
				"container": container,
			})
			continue
		}

		if !ok {
			c.Logger.Info("container-provider-container-not-found", lager.Data{
				"container": container,
			})
			continue
		}

		c.Logger.Debug("completed-deleting-container", lager.Data{
			"container": container,
		})
	}

	c.Logger.Debug("completed-deleting-containers")

	return nil
}
