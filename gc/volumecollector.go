package gc

import (
	"errors"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/metric"
	"github.com/concourse/atc/worker"
	"github.com/concourse/baggageclaim"
)

var volumeCollectorFailedErr = errors.New("volume collector failed")

type volumeCollector struct {
	rootLogger    lager.Logger
	volumeFactory db.VolumeFactory
	jobRunner     WorkerJobRunner
}

func NewVolumeCollector(
	logger lager.Logger,
	volumeFactory db.VolumeFactory,
	jobRunner WorkerJobRunner,
) Collector {
	return &volumeCollector{
		rootLogger:    logger,
		volumeFactory: volumeFactory,
		jobRunner:     jobRunner,
	}
}

func (vc *volumeCollector) Run() error {
	logger := vc.rootLogger.Session("run")

	logger.Debug("start")
	defer logger.Debug("done")

	var err error

	orphanedErr := vc.cleanupOrphanedVolumes(logger.Session("orphaned-volumes"))
	if orphanedErr != nil {
		vc.rootLogger.Error("volume-collector", orphanedErr)
		err = volumeCollectorFailedErr
	}

	failedErr := vc.cleanupFailedVolumes(logger.Session("failed-volumes"))
	if failedErr != nil {
		vc.rootLogger.Error("volume-collector", failedErr)
		err = volumeCollectorFailedErr
	}

	return err
}

func (vc *volumeCollector) cleanupFailedVolumes(logger lager.Logger) error {

	failedVolumes, err := vc.volumeFactory.GetFailedVolumes()
	if err != nil {
		logger.Error("failed-to-get-failed-volumes", err)
		return err
	}

	if len(failedVolumes) > 0 {
		logger.Debug("found-failed-volumes", lager.Data{
			"failed": len(failedVolumes),
		})
	}

	metric.FailedVolumesToBeGarbageCollected{
		Volumes: len(failedVolumes),
	}.Emit(logger)

	for _, failedVolume := range failedVolumes {
		destroyDBVolume(logger, failedVolume)
	}

	return nil
}

func (vc *volumeCollector) cleanupOrphanedVolumes(logger lager.Logger) error {
	createdVolumes, destroyingVolumes, err := vc.volumeFactory.GetOrphanedVolumes()
	if err != nil {
		logger.Error("failed-to-get-orphaned-volumes", err)
		return err
	}

	if len(createdVolumes) > 0 || len(destroyingVolumes) > 0 {
		logger.Debug("found-orphaned-volumes", lager.Data{
			"created":    len(createdVolumes),
			"destroying": len(destroyingVolumes),
		})
	}

	metric.CreatedVolumesToBeGarbageCollected{
		Volumes: len(createdVolumes),
	}.Emit(logger)

	metric.DestroyingVolumesToBeGarbageCollected{
		Volumes: len(destroyingVolumes),
	}.Emit(logger)

	for _, createdVolume := range createdVolumes {
		// queue
		vLog := logger.Session("mark-created-as-destroying", lager.Data{
			"volume": createdVolume.Handle(),
			"worker": createdVolume.WorkerName(),
		})

		destroyingVolume, err := createdVolume.Destroying()
		if err != nil {
			vLog.Error("failed-to-transition", err)
			continue
		}

		destroyingVolumes = append(destroyingVolumes, destroyingVolume)
	}

	for _, destroyingVolume := range destroyingVolumes {
		// chuck volume into worker pool

		vLog := logger.Session("destroy", lager.Data{
			"handle": destroyingVolume.Handle(),
			"worker": destroyingVolume.WorkerName(),
		})

		vc.jobRunner.Try(logger,
			destroyingVolume.WorkerName(),
			&job{
				JobName: destroyingVolume.Handle(),
				RunFunc: destroyDestroyingVolume(vLog, destroyingVolume),
			},
		)
	}

	return nil
}

func destroyDestroyingVolume(logger lager.Logger, destroyingVolume db.DestroyingVolume) func(worker.Worker) {
	return func(workerClient worker.Worker) {
		baggageClaimClient := workerClient.BaggageclaimClient()
		if baggageClaimClient == nil {
			logger.Info("baggageclaim-client-is-missing")
			return
		}

		volume, found, err := baggageClaimClient.LookupVolume(logger, destroyingVolume.Handle())
		if err != nil {
			logger.Error("failed-to-lookup-volume-in-baggageclaim", err)
			return
		}

		if destroyRealVolume(logger.Session("in-worker"), volume, workerClient.Name(), found) {
			destroyDBVolume(logger.Session("in-db"), destroyingVolume)
		}
	}
}

func destroyRealVolume(logger lager.Logger, volume baggageclaim.Volume, workerName string, found bool) bool {
	if found {
		logger.Debug("destroying")

		err := volume.Destroy()
		if err != nil {
			logger.Error("failed-to-destroy", err)
			return false
		}

		logger.Debug("destroyed")

		metric.VolumesDeleted.Inc()
	} else {
		logger.Debug("already-removed")
	}

	return true
}

func destroyDBVolume(logger lager.Logger, dbVolume db.DestroyingVolume) {
	logger.Debug("destroying")

	destroyed, err := dbVolume.Destroy()
	if err != nil {
		logger.Error("failed-to-destroy", err)
		return
	}

	if !destroyed {
		logger.Info("could-not-destroy")
		return
	}

	logger.Debug("destroyed")
}
