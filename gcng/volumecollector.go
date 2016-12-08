package gcng

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/worker/transport"
	bclient "github.com/concourse/baggageclaim/client"
)

type volumeCollector struct {
	logger                    lager.Logger
	volumeFactory             dbng.VolumeFactory
	baggageclaimClientFactory BaggageclaimClientFactory
}

//go:generate counterfeiter . BaggageclaimClientFactory

type BaggageclaimClientFactory interface {
	NewClient(apiURL string, workerName string) bclient.Client
}

type baggageclaimClientFactory struct {
	dbWorkerFactory dbng.WorkerFactory
}

func NewBaggageclaimClientFactory(dbWorkerFactory dbng.WorkerFactory) BaggageclaimClientFactory {
	return &baggageclaimClientFactory{
		dbWorkerFactory: dbWorkerFactory,
	}
}

func (f *baggageclaimClientFactory) NewClient(apiURL string, workerName string) bclient.Client {
	rountTripper := transport.NewBaggageclaimRoundTripper(
		workerName,
		&apiURL,
		f.dbWorkerFactory,
		&http.Transport{DisableKeepAlives: true},
	)
	return bclient.New(apiURL, rountTripper)
}

func NewVolumeCollector(
	logger lager.Logger,
	volumeFactory dbng.VolumeFactory,
	baggageclaimClientFactory BaggageclaimClientFactory,
) Collector {
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
			continue
		}

		destroyingVolumes = append(destroyingVolumes, destroyingVolume)
	}

	for _, destroyingVolume := range destroyingVolumes {
		vLog := vc.logger.Session("destroy", lager.Data{
			"handle": destroyingVolume.Handle(),
			"worker": destroyingVolume.Worker().Name,
		})

		if destroyingVolume.Worker().BaggageclaimURL == nil {
			vLog.Debug("baggageclaim-url-is-missing", lager.Data{"worker-name": destroyingVolume.Worker().Name})
			continue
		}

		baggageclaimClient := vc.baggageclaimClientFactory.NewClient(*destroyingVolume.Worker().BaggageclaimURL, destroyingVolume.Worker().Name)
		volume, found, err := baggageclaimClient.LookupVolume(vc.logger, destroyingVolume.Handle())
		if err != nil {
			vLog.Error("failed-to-lookup-volume-in-baggageclaim", err)
			continue
		}

		if found {
			vLog.Debug("destroying-baggageclaim-volume")
			err := volume.Destroy()
			if err != nil {
				vLog.Error("failed-to-destroy-baggageclaim-volume", err)
				continue
			}
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
