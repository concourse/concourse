package gc

import (
	"context"
	"errors"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/metric"
	multierror "github.com/hashicorp/go-multierror"
)

var volumeCollectorFailedErr = errors.New("volume collector failed")

type volumeCollector struct {
	volumeRepository db.VolumeRepository
	jobRunner        WorkerJobRunner
}

func NewVolumeCollector(volumeRepository db.VolumeRepository, jobRunner WorkerJobRunner) Collector {
	return &volumeCollector{
		volumeRepository: volumeRepository,
		jobRunner:        jobRunner,
	}
}

func (vc *volumeCollector) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("volume-collector")

	logger.Debug("start")
	defer logger.Debug("done")

	var errs error

	err := vc.markOrphanedVolumesasDestroying(logger.Session("orphaned-volumes"))
	if err != nil {
		errs = multierror.Append(errs, err)
		logger.Error("failed-to-mark-orphaned-volumes-as-destroying", err)
	}

	err = vc.cleanupFailedVolumes(logger.Session("failed-volumes"))
	if err != nil {
		errs = multierror.Append(errs, err)
		logger.Error("failed-to-clean-up-failed-volumes", err)
	}

	return errs
}

func (vc *volumeCollector) cleanupFailedVolumes(logger lager.Logger) error {
	failedVolumes, err := vc.volumeRepository.GetFailedVolumes()
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

func (vc *volumeCollector) markOrphanedVolumesasDestroying(logger lager.Logger) error {
	createdVolumes, destroyingVolumes, err := vc.volumeRepository.GetOrphanedVolumes()
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

		_, err := createdVolume.Destroying()
		if err != nil {
			vLog.Error("failed-to-transition", err)
			continue
		}
	}
	return nil
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
