package gc

import (
	"context"
	"errors"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/metric"
	"github.com/concourse/atc/worker"
	multierror "github.com/hashicorp/go-multierror"
)

var volumeCollectorFailedErr = errors.New("volume collector failed")

type volumeCollector struct {
	volumeFactory db.VolumeFactory
	jobRunner     WorkerJobRunner
}

func NewVolumeCollector(volumeFactory db.VolumeFactory, jobRunner WorkerJobRunner) Collector {
	return &volumeCollector{
		volumeFactory: volumeFactory,
		jobRunner:     jobRunner,
	}
}

func (vc *volumeCollector) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("volume-collector")

	logger.Debug("start")
	defer logger.Debug("done")

	var errs error

	err := vc.cleanupOrphanedVolumes(logger.Session("orphaned-volumes"))
	if err != nil {
		errs = multierror.Append(errs, err)
		logger.Error("failed-to-clean-up-orphaned-volumes", err)
	}

	err = vc.cleanupFailedVolumes(logger.Session("failed-volumes"))
	if err != nil {
		errs = multierror.Append(errs, err)
		logger.Error("failed-to-clean-up-failed-volumes", err)
	}

	return errs
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

	workerVolume := make(map[string][]db.DestroyingVolume)
	for _, destroyingVolume := range destroyingVolumes {
		volumes, ok := workerVolume[destroyingVolume.WorkerName()]
		if ok {
			// update existing array
			volumes = append(volumes, destroyingVolume)
			workerVolume[destroyingVolume.WorkerName()] = volumes
		} else {
			// create new array
			workerVolume[destroyingVolume.WorkerName()] = []db.DestroyingVolume{destroyingVolume}
		}
	}

	for workerName, volumes := range workerVolume {
		vLog := logger.Session("destroy", lager.Data{
			"worker": workerName,
		})
		vc.jobRunner.Try(logger,
			workerName,
			&job{
				JobName: workerName,
				RunFunc: destroyDestroyingVolumes(vLog, volumes),
			},
		)
	}

	return nil
}

func destroyDestroyingVolumes(logger lager.Logger, destroyingVolumes []db.DestroyingVolume) func(worker.Worker) {
	return func(workerClient worker.Worker) {
		baggageClaimClient := workerClient.BaggageclaimClient()
		if baggageClaimClient == nil {
			logger.Info("baggageclaim-client-is-missing")
			return
		}

		var handles []string
		for _, volume := range destroyingVolumes {
			handles = append(handles, volume.Handle())
		}

		err := baggageClaimClient.DestroyVolumes(logger, handles)
		if err != nil {
			logger.Error("failed-to-destroy-volumes-in-baggageclaim", err)
			return
		} else {
			for _, dbVolume := range destroyingVolumes {
				metric.VolumesDeleted.Inc()
				destroyDBVolume(logger.Session("in-db", lager.Data{"volume-handle": dbVolume.Handle()}), dbVolume)
			}
		}
	}
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
