package worker

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/atc/metric"
	"github.com/concourse/baggageclaim"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
)

var ErrMissingVolume = errors.New("volume mounted to container is missing")

type gardenWorkerContainer struct {
	garden.Container

	gardenClient garden.Client
	db           GardenWorkerDB

	volumes      []Volume
	volumeMounts []VolumeMount

	user string

	clock clock.Clock

	release      chan *time.Duration
	heartbeating *sync.WaitGroup

	releaseOnce sync.Once
}

func newGardenWorkerContainer(
	logger lager.Logger,
	container garden.Container,
	gardenClient garden.Client,
	baggageclaimClient baggageclaim.Client,
	db GardenWorkerDB,
	clock clock.Clock,
	volumeFactory VolumeFactory,
) (Container, error) {
	logger = logger.WithData(lager.Data{"container": container.Handle()})

	workerContainer := &gardenWorkerContainer{
		Container: container,

		gardenClient: gardenClient,
		db:           db,

		clock: clock,

		heartbeating: new(sync.WaitGroup),
		release:      make(chan *time.Duration, 1),
	}

	workerContainer.heartbeat(logger.Session("initial-heartbeat"), ContainerTTL)

	workerContainer.heartbeating.Add(1)
	go workerContainer.heartbeatContinuously(
		logger.Session("continuous-heartbeat"),
		clock.NewTicker(containerKeepalive),
	)

	metric.TrackedContainers.Inc()

	properties, err := workerContainer.Properties()
	if err != nil {
		workerContainer.Release(nil)
		return nil, err
	}

	err = workerContainer.initializeVolumes(logger, properties, baggageclaimClient, volumeFactory)
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

func (container *gardenWorkerContainer) Release(finalTTL *time.Duration) {
	container.releaseOnce.Do(func() {
		container.release <- finalTTL
		container.heartbeating.Wait()
		metric.TrackedContainers.Dec()

		for _, v := range container.volumes {
			v.Release(finalTTL)
		}
	})
}

func (container *gardenWorkerContainer) Run(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	spec.User = container.user
	return container.Container.Run(spec, io)
}

func (container *gardenWorkerContainer) Volumes() []Volume {
	return container.volumes
}

func (container *gardenWorkerContainer) VolumeMounts() []VolumeMount {
	return container.volumeMounts
}

func (container *gardenWorkerContainer) initializeVolumes(
	logger lager.Logger,
	properties garden.Properties,
	baggageclaimClient baggageclaim.Client,
	volumeFactory VolumeFactory,
) error {
	if baggageclaimClient == nil {
		return nil
	}

	volumesByHandle := map[string]Volume{}
	handlesJSON, found := properties[volumePropertyName]
	var err error
	if found {
		volumesByHandle, err = container.setVolumes(logger, handlesJSON, baggageclaimClient, volumeFactory)
		if err != nil {
			return err
		}
	}

	mountsJSON, found := properties[volumeMountsPropertyName]
	if found {
		var handleToMountPath map[string]string
		err := json.Unmarshal([]byte(mountsJSON), &handleToMountPath)
		if err != nil {
			return err
		}

		volumeMounts := []VolumeMount{}
		for h, m := range handleToMountPath {
			volumeMounts = append(volumeMounts, VolumeMount{
				Volume:    volumesByHandle[h],
				MountPath: m,
			})
		}

		container.volumeMounts = volumeMounts
	}

	return nil
}

func (container *gardenWorkerContainer) setVolumes(
	logger lager.Logger,
	handlesJSON string,
	baggageclaimClient baggageclaim.Client,
	volumeFactory VolumeFactory,
) (map[string]Volume, error) {
	volumesByHandle := map[string]Volume{}

	var handles []string
	err := json.Unmarshal([]byte(handlesJSON), &handles)
	if err != nil {
		return nil, err
	}

	volumes := []Volume{}
	for _, h := range handles {
		volumeLogger := logger.Session("volume", lager.Data{
			"handle": h,
		})

		baggageClaimVolume, volumeFound, err := baggageclaimClient.LookupVolume(logger, h)
		if err != nil {
			volumeLogger.Error("failed-to-lookup-volume", err)
			return nil, err
		}

		if !volumeFound {
			volumeLogger.Error("volume-is-missing-on-worker", ErrMissingVolume)
			return nil, ErrMissingVolume
		}

		volume, volumeFound, err := volumeFactory.Build(volumeLogger, baggageClaimVolume)
		if err != nil {
			volumeLogger.Error("failed-to-build-volume", nil)
			return nil, err
		}

		if !volumeFound {
			volumeLogger.Error("volume-is-missing-in-database", ErrMissingVolume)
			return nil, ErrMissingVolume
		}

		volumes = append(volumes, volume)

		volumesByHandle[h] = volume
	}

	container.volumes = volumes
	return volumesByHandle, nil
}

func (container *gardenWorkerContainer) heartbeatContinuously(logger lager.Logger, pacemaker clock.Ticker) {
	defer container.heartbeating.Done()
	defer pacemaker.Stop()

	logger.Debug("start")
	defer logger.Debug("done")

	for {
		select {
		case <-pacemaker.C():
			container.heartbeat(logger.Session("tick"), ContainerTTL) // TODO: how will this not overwrite the infinite TTL we set?

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
