package worker

import (
	"errors"
	"sync"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/baggageclaim"
)

var ErrMissingVolume = errors.New("volume mounted to container is missing")

type gardenWorkerContainer struct {
	garden.Container
	dbContainer dbng.CreatedContainer
	dbVolumes   []dbng.CreatedVolume

	gardenClient garden.Client
	db           GardenWorkerDB

	volumeMounts []VolumeMount

	user string

	releaseOnce sync.Once

	workerName string
}

func newGardenWorkerContainer(
	logger lager.Logger,
	container garden.Container,
	dbContainer dbng.CreatedContainer,
	dbContainerVolumes []dbng.CreatedVolume,
	gardenClient garden.Client,
	baggageclaimClient baggageclaim.Client,
	db GardenWorkerDB,
	workerName string,
) (Container, error) {
	logger = logger.WithData(lager.Data{"container": container.Handle()})

	workerContainer := &gardenWorkerContainer{
		Container:   container,
		dbContainer: dbContainer,
		dbVolumes:   dbContainerVolumes,

		gardenClient: gardenClient,
		db:           db,

		workerName: workerName,
	}

	err := workerContainer.initializeVolumes(logger, baggageclaimClient)
	if err != nil {
		return nil, err
	}

	properties, err := workerContainer.Properties()
	if err != nil {
		return nil, err
	}

	if properties["user"] != "" {
		workerContainer.user = properties["user"]
	} else {
		workerContainer.user = "root"
	}

	return workerContainer, nil
}

func (container *gardenWorkerContainer) Destroy() error {
	return container.gardenClient.Destroy(container.Handle())
}

func (container *gardenWorkerContainer) WorkerName() string {
	return container.workerName
}

func (container *gardenWorkerContainer) MarkAsHijacked() error {
	return container.dbContainer.MarkAsHijacked()
}

func (container *gardenWorkerContainer) Run(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	spec.User = container.user
	return container.Container.Run(spec, io)
}

func (container *gardenWorkerContainer) VolumeMounts() []VolumeMount {
	return container.volumeMounts
}

func (container *gardenWorkerContainer) initializeVolumes(
	logger lager.Logger,
	baggageclaimClient baggageclaim.Client,
) error {

	volumeMounts := []VolumeMount{}

	for _, dbVolume := range container.dbVolumes {
		volumeLogger := logger.Session("volume", lager.Data{
			"handle": dbVolume.Handle(),
		})

		baggageClaimVolume, volumeFound, err := baggageclaimClient.LookupVolume(logger, dbVolume.Handle())
		if err != nil {
			volumeLogger.Error("failed-to-lookup-volume", err)
			return err
		}

		if !volumeFound {
			volumeLogger.Error("volume-is-missing-on-worker", ErrMissingVolume, lager.Data{"handle": dbVolume.Handle()})
			return errors.New("volume mounted to container is missing " + dbVolume.Handle() + " from worker " + container.workerName)
		}

		volumeMounts = append(volumeMounts, VolumeMount{
			Volume:    NewVolume(baggageClaimVolume, dbVolume),
			MountPath: dbVolume.Path(),
		})
	}

	container.volumeMounts = volumeMounts

	return nil
}
