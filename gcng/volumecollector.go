package gcng

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/dbng"
	bclient "github.com/concourse/baggageclaim/client"
)

type VolumeCollector interface {
	Run() error
}

type volumeCollector struct {
	logger                    lager.Logger
	volumeFactory             dbng.VolumeFactory
	baggageclaimClientFactory BaggageclaimClientFactory
}

//go:generate counterfeiter . BaggageclaimClientFactory

type BaggageclaimClientFactory interface {
	NewClient(apiURL string) bclient.Client
}

type baggageclaimClientFactory struct{}

func NewBaggageclaimClientFactory() BaggageclaimClientFactory {
	return &baggageclaimClientFactory{}
}

func (f *baggageclaimClientFactory) NewClient(apiURL string) bclient.Client {
	return bclient.New(apiURL)
}

func NewVolumeCollector(
	logger lager.Logger,
	volumeFactory dbng.VolumeFactory,
	baggageclaimClientFactory BaggageclaimClientFactory,
) VolumeCollector {
	return &volumeCollector{
		logger:                    logger,
		volumeFactory:             volumeFactory,
		baggageclaimClientFactory: baggageclaimClientFactory,
	}
}

func (vc *volumeCollector) Run() error {
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

		baggageclaimClient := vc.baggageclaimClientFactory.NewClient(destroyingVolume.Worker().BaggageclaimURL)
		volume, found, err := baggageclaimClient.LookupVolume(vc.logger, destroyingVolume.Handle())
		if err != nil {
			vLog.Error("failed-to-lookup-volume-in-baggageclaim", err)
			continue
		}

		if found {
			vLog.Debug("destroying-baggageclaim-volume")
			volume.Destroy()
		} else {
			vLog.Debug("volume-already-removed-from-baggageclaim")
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
