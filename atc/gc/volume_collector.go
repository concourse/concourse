package gc

import (
	"context"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
	multierror "github.com/hashicorp/go-multierror"
)

type volumeCollector struct {
	volumeRepository         db.VolumeRepository
	missingVolumeGracePeriod time.Duration
}

func NewVolumeCollector(
	volumeRepository db.VolumeRepository,
	missingVolumeGracePeriod time.Duration,
) *volumeCollector {
	return &volumeCollector{
		volumeRepository:         volumeRepository,
		missingVolumeGracePeriod: missingVolumeGracePeriod,
	}
}

func (vc *volumeCollector) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("volume-collector")

	logger.Debug("start")
	defer logger.Debug("done")

	start := time.Now()
	defer func() {
		metric.VolumeCollectorDuration{
			Duration: time.Since(start),
		}.Emit(logger)
	}()

	var errs error

	err := vc.cleanupFailedVolumes(logger.Session("failed-volumes"))
	if err != nil {
		errs = multierror.Append(errs, err)
		logger.Error("failed-to-clean-up-failed-volumes", err)
	}

	err = vc.markOrphanedVolumesAsDestroying(logger.Session("mark-volumes"))
	if err != nil {
		errs = multierror.Append(errs, err)
		logger.Error("failed-to-transition-created-volumes-to-destroying", err)
	}

	_, err = vc.volumeRepository.RemoveMissingVolumes(vc.missingVolumeGracePeriod)
	if err != nil {
		errs = multierror.Append(errs, err)
		logger.Error("failed-to-clean-up-missing-volumes", err)
	}

	return errs
}

func (vc *volumeCollector) cleanupFailedVolumes(logger lager.Logger) error {
	failedVolumesLen, err := vc.volumeRepository.DestroyFailedVolumes()
	if err != nil {
		logger.Error("failed-to-get-failed-volumes", err)
		return err
	}

	if failedVolumesLen > 0 {
		logger.Debug("found-failed-volumes", lager.Data{
			"failed": failedVolumesLen,
		})
	}

	metric.FailedVolumesToBeGarbageCollected{
		Volumes: failedVolumesLen,
	}.Emit(logger)

	return nil
}

func (vc *volumeCollector) markOrphanedVolumesAsDestroying(logger lager.Logger) error {
	orphanedVolumesHandles, err := vc.volumeRepository.GetOrphanedVolumes()
	if err != nil {
		logger.Error("failed-to-get-orphaned-volumes", err)
		return err
	}

	if len(orphanedVolumesHandles) > 0 {
		logger.Debug("found-orphaned-volumes", lager.Data{
			"destroying": len(orphanedVolumesHandles),
		})
	}

	metric.CreatedVolumesToBeGarbageCollected{
		Volumes: len(orphanedVolumesHandles),
	}.Emit(logger)

	for _, orphanedVolume := range orphanedVolumesHandles {
		// queue
		vLog := logger.Session("mark-created-as-destroying", lager.Data{
			"volume": orphanedVolume.Handle(),
			"worker": orphanedVolume.WorkerName(),
		})

		_, err = orphanedVolume.Destroying()
		if err != nil {
			vLog.Error("failed-to-transition", err)
			continue
		}

	}

	return nil
}
