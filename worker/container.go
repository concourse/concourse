package worker

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/atc/metric"
	"github.com/concourse/baggageclaim"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
)

type gardenWorkerContainer struct {
	garden.Container

	gardenClient garden.Client
	db           GardenWorkerDB

	volumes []Volume

	clock clock.Clock

	release      chan time.Duration
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
	workerContainer := &gardenWorkerContainer{
		Container: container,

		gardenClient: gardenClient,
		db:           db,

		clock: clock,

		heartbeating: new(sync.WaitGroup),
		release:      make(chan time.Duration, 1),
	}

	workerContainer.heartbeat(logger.Session("initial-heartbeat"), containerTTL)

	workerContainer.heartbeating.Add(1)
	go workerContainer.heartbeatContinuously(
		logger.Session("continuous-heartbeat"),
		clock.NewTicker(containerKeepalive),
	)

	metric.TrackedContainers.Inc()

	properties, err := workerContainer.Properties()
	if err != nil {
		workerContainer.Release(0)
		return nil, err
	}

	err = workerContainer.initializeVolumes(logger, properties, baggageclaimClient, volumeFactory)
	if err != nil {
		workerContainer.Release(0)
		return nil, err
	}

	return workerContainer, nil
}

func (container *gardenWorkerContainer) Destroy() error {
	container.Release(0)
	return container.gardenClient.Destroy(container.Handle())
}

func (container *gardenWorkerContainer) Release(finalTTL time.Duration) {
	container.releaseOnce.Do(func() {
		container.release <- finalTTL
		container.heartbeating.Wait()
		metric.TrackedContainers.Dec()

		for _, v := range container.volumes {
			v.Release(finalTTL)
		}
	})
}

func (container *gardenWorkerContainer) Volumes() []Volume {
	return container.volumes
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

	handlesJSON, found := properties[volumePropertyName]
	if !found {
		container.volumes = []Volume{}
		return nil
	}

	var handles []string
	err := json.Unmarshal([]byte(handlesJSON), &handles)
	if err != nil {
		return err
	}

	volumes := []Volume{}
	for _, h := range handles {
		baggageClaimVolume, err := baggageclaimClient.LookupVolume(logger, h)
		if err != nil {
			return err
		}

		volume, err := volumeFactory.Build(baggageClaimVolume)
		if err != nil {
			return err
		}

		volumes = append(volumes, volume)
	}

	container.volumes = volumes

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
			container.heartbeat(logger.Session("tick"), containerTTL)

		case finalTTL := <-container.release:
			if finalTTL != 0 {
				container.heartbeat(logger.Session("final"), finalTTL)
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
