package gcng

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/worker"
)

type VolumeCollector interface {
	Run() error
}

type volumeCollector struct {
	logger        lager.Logger
	volumeFactory dbng.VolumeFactory
	workerClient  worker.Client
}

func NewVolumeCollector(
	logger lager.Logger,
	volumeFactory dbng.VolumeFactory,
	workerClient worker.Client,
) VolumeCollector {
	return &volumeCollector{
		logger:        logger,
		volumeFactory: volumeFactory,
		workerClient:  workerClient,
	}
}

func (vc *volumeCollector) Run() error {
	workers, err := vc.workerClient.Workers()
	if err != nil {
		vc.logger.Error("failed-to-get-workers", err)
		return err
	}

	workersMap := map[string]worker.Worker{}
	for _, worker := range workers {
		workersMap[worker.Name()] = worker
	}

	createdVolumes, destroyingVolumes, err := vc.volumeFactory.GetOrphanedVolumes()
	if err != nil {
		vc.logger.Error("failed-to-get-orphaned-volumes", err)
		return err
	}

	for _, createdVolume := range createdVolumes {
		destroyingVolume, err := createdVolume.Destroying()
		if err != nil {
			vc.logger.Error("failed-to-mark-volume-as-destroying", err)
			return err
		}

		destroyingVolumes = append(destroyingVolumes, destroyingVolume)
	}

	for _, destroyingVolume := range destroyingVolumes {
		vLog := vc.logger.Session("destroy", lager.Data{
			"handle": destroyingVolume.Handle(),
			"worker": destroyingVolume.Worker().Name,
		})

		volumeWorker, ok := workersMap[destroyingVolume.Worker().Name]
		if !ok {
			vLog.Info("could-not-locate-worker")
			continue
		}

		volume, found, err := volumeWorker.LookupVolume(vc.logger, destroyingVolume.Handle())
		if err != nil {
			vLog.Error("failed-to-lookup-volume", err)
			continue
		}

		if found {
			vLog.Debug("destroying-worker-volume")
			volume.Destroy()
		} else {
			vLog.Debug("volume-already-removed-from-worker")
		}

		vLog.Debug("destroying-db-volume")

		destroyed, err := destroyingVolume.Destroy()
		if err != nil {
			vc.logger.Error("failed-to-destroy-volume-in-db", err)
			continue
		}

		if !destroyed {
			vLog.Info("could-not-destroy-volume-in-db")
			continue
		}
	}

	return nil
}
