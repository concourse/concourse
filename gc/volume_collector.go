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
}

func NewVolumeCollector(volumeRepository db.VolumeRepository) Collector {
	return &volumeCollector{
		volumeRepository: volumeRepository,
	}
}

func (vc *volumeCollector) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("volume-collector")

	logger.Debug("start")
	defer logger.Debug("done")

	var errs error

	err := vc.cleanupFailedVolumes(logger.Session("failed-volumes"))
	if err != nil {
		errs = multierror.Append(errs, err)
		logger.Error("failed-to-clean-up-failed-volumes", err)
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
