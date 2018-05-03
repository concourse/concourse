package gc

import (
	"errors"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/metric"
)

//go:generate counterfeiter . ContainerDestroyer

// ContainerDestroyer allows the caller to remove containers from the Concourse ContainerRepository
type ContainerDestroyer interface {
	Destroy(workerName string, handles []string) error
}

type containerDestroyer struct {
	logger              lager.Logger
	containerRepository db.ContainerRepository
}

// NewContainerDestroyer provides a constructor for a ContainerDestroyer interface implementation
func NewContainerDestroyer(
	logger lager.Logger,
	containerRepository db.ContainerRepository,
) ContainerDestroyer {
	return &containerDestroyer{
		logger:              logger,
		containerRepository: containerRepository,
	}
}

func (d *containerDestroyer) Destroy(workerName string, currentHandles []string) error {

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
