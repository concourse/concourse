package worker

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/v5/atc/db"
)

//go:generate counterfeiter . Client

type Client interface {
	FindContainer(logger lager.Logger, teamID int, handle string) (Container, bool, error)
	FindVolume(logger lager.Logger, teamID int, handle string) (Volume, bool, error)
	CreateVolume(logger lager.Logger, spec VolumeSpec, teamID int, volumeType db.VolumeType) (Volume, error)
}

func NewClient(pool Pool, provider WorkerProvider) *client {
	return &client{
		pool:     pool,
		provider: provider,
	}
}

type client struct {
	pool     Pool
	provider WorkerProvider
}

func (client *client) FindContainer(logger lager.Logger, teamID int, handle string) (Container, bool, error) {
	worker, found, err := client.provider.FindWorkerForContainer(
		logger.Session("find-worker"),
		teamID,
		handle,
	)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	return worker.FindContainerByHandle(logger, teamID, handle)
}

func (client *client) FindVolume(logger lager.Logger, teamID int, handle string) (Volume, bool, error) {
	worker, found, err := client.provider.FindWorkerForVolume(
		logger.Session("find-worker"),
		teamID,
		handle,
	)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	return worker.LookupVolume(logger, handle)
}

func (client *client) CreateVolume(logger lager.Logger, spec VolumeSpec, teamID int, volumeType db.VolumeType) (Volume, error) {
	worker, err := client.pool.FindOrChooseWorker(logger, WorkerSpec{TeamID: teamID})
	if err != nil {
		return nil, err
	}

	return worker.CreateVolume(logger, spec, teamID, volumeType)
}
