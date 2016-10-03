package worker

import (
	"errors"
	"sync"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/metric"
	"github.com/concourse/baggageclaim"
)

var ErrMissingVolume = errors.New("volume mounted to container is missing")

type gardenWorkerContainer struct {
	garden.Container
	dbContainer *dbng.CreatedContainer

	gardenClient garden.Client
	db           GardenWorkerDB

	volumeMounts []VolumeMount

	user string

	clock clock.Clock

	release      chan *time.Duration
	heartbeating *sync.WaitGroup

	releaseOnce sync.Once

	workerName string
}

func newGardenWorkerContainer(
	logger lager.Logger,
	container garden.Container,
	dbContainer *dbng.CreatedContainer,
	gardenClient garden.Client,
	baggageclaimClient baggageclaim.Client,
	db GardenWorkerDB,
	clock clock.Clock,
	volumeFactory VolumeFactory,
	workerName string,
) (Container, error) {
	logger = logger.WithData(lager.Data{"container": container.Handle()})

	workerContainer := &gardenWorkerContainer{
		Container:   container,
		dbContainer: dbContainer,

		gardenClient: gardenClient,
		db:           db,

		clock: clock,

		heartbeating: new(sync.WaitGroup),
		release:      make(chan *time.Duration, 1),
		workerName:   workerName,
	}

	workerContainer.heartbeat(logger.Session("initial-heartbeat"), ContainerTTL)

	workerContainer.heartbeating.Add(1)
	go workerContainer.heartbeatContinuously(
		logger.Session("continuous-heartbeat"),
		clock.NewTicker(containerKeepalive),
	)

	metric.TrackedContainers.Inc()

	err := workerContainer.initializeVolumes(logger, baggageclaimClient, volumeFactory)
	if err != nil {
		workerContainer.Release(nil)
		return nil, err
	}

	properties, err := workerContainer.Properties()
	if err != nil {
		workerContainer.Release(nil)
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
	container.Release(nil)
	return container.gardenClient.Destroy(container.Handle())
}

func (container *gardenWorkerContainer) WorkerName() string {
	return container.workerName
}

func (container *gardenWorkerContainer) Release(finalTTL *time.Duration) {
	container.releaseOnce.Do(func() {
		container.release <- finalTTL
		container.heartbeating.Wait()
		metric.TrackedContainers.Dec()
	})
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
	volumeFactory VolumeFactory,
) error {
	volumes, err := container.dbContainer.Volumes()
	if err != nil {
		return err
	}

	volumeMounts := []VolumeMount{}

	for _, volume := range volumes {
		volumeLogger := logger.Session("volume", lager.Data{
			"handle": volume.Handle(),
		})

		baggageClaimVolume, volumeFound, err := baggageclaimClient.LookupVolume(logger, volume.Handle())
		if err != nil {
			volumeLogger.Error("failed-to-lookup-volume", err)
			return err
		}

		if !volumeFound {
			volumeLogger.Error("volume-is-missing-on-worker", ErrMissingVolume, lager.Data{"handle": volume.Handle()})
			return errors.New("volume mounted to container is missing " + volume.Handle())
		}

		volume, err := volumeFactory.BuildWithIndefiniteTTL(volumeLogger, baggageClaimVolume)
		if err != nil {
			volumeLogger.Error("failed-to-build-volume", nil)
			return err
		}

		volumeMounts = append(volumeMounts, VolumeMount{
			Volume:    volume,
			MountPath: volume.Path(),
		})
	}

	container.volumeMounts = volumeMounts

	return nil
}

func (container *gardenWorkerContainer) heartbeatContinuously(logger lager.Logger, pacemaker clock.Ticker) {
	defer container.heartbeating.Done()
	defer pacemaker.Stop()

	logger.Debug("start")
	defer logger.Debug("done")

	for {
		select {
		case <-pacemaker.C():
			container.heartbeat(logger.Session("tick"), ContainerTTL)

		case finalTTL := <-container.release:
			if finalTTL != nil {
				container.heartbeat(logger.Session("final"), *finalTTL)
			}

			return
		}
	}
}

func (container *gardenWorkerContainer) heartbeat(logger lager.Logger, ttl time.Duration) {
	logger.Debug("start")
	defer logger.Debug("done")

	err := container.db.UpdateExpiresAtOnContainer(container.Handle(), ttl)
	if err != nil {
		logger.Error("failed-to-heartbeat-to-db", err)
	}

	err = container.SetGraceTime(ttl)
	if err != nil {
		logger.Error("failed-to-heartbeat-to-container", err)
	}
}
