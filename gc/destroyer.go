package gc

import (
	"errors"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/metric"
)

//go:generate counterfeiter . Destroyer

// Destroyer allows the caller to remove containers and volumes
type Destroyer interface {
	DestroyContainers(workerName string, handles []string) error
	DestroyVolumes(workerName string, handles []string) error
}

type destroyer struct {
	logger              lager.Logger
	containerRepository db.ContainerRepository
	volumeRepository    db.VolumeRepository
}

// NewDestroyer provides a constructor for a Destroyer interface implementation
func NewDestroyer(
	logger lager.Logger,
	containerRepository db.ContainerRepository,
	volumeRepository db.VolumeRepository,
) Destroyer {
	return &destroyer{
		logger:              logger,
		containerRepository: containerRepository,
		volumeRepository:    volumeRepository,
	}
}

func (d *destroyer) DestroyContainers(workerName string, currentHandles []string) error {

	if workerName != "" {
		if currentHandles != nil {
			deleted, err := d.containerRepository.RemoveDestroyingContainers(workerName, currentHandles)
			if err != nil {
				d.logger.Error("failed-to-destroy-containers", err)
				return err
			}

			for i := 0; i < deleted; i++ {
				metric.ContainersDeleted.Inc()
			}
		}
		return nil
	}

	err := errors.New("worker-name-must-be-provided")
	d.logger.Error("failed-to-destroy-containers", err)

	return err
}

func (d *destroyer) DestroyVolumes(workerName string, currentHandles []string) error {

	if workerName != "" {
		if currentHandles != nil {
			deleted, err := d.volumeRepository.RemoveDestroyingVolumes(workerName, currentHandles)
			if err != nil {
				d.logger.Error("failed-to-destroy-volumes", err)
				return err
			}

			for i := 0; i < deleted; i++ {
				metric.VolumesDeleted.Inc()
			}
		}
		return nil
	}

	err := errors.New("worker-name-must-be-provided")
	d.logger.Error("failed-to-destroy-volumes", err)

	return err
}
