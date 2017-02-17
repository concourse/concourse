package gcng

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/worker/transport"
	"github.com/concourse/baggageclaim"
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
		vLog := vc.logger.Session("mark-destroying", lager.Data{
			"volume": createdVolume.Handle(),
			"worker": createdVolume.Worker().Name,
		})

		destroyingVolume, err := createdVolume.Destroying()
		if err != nil {
			vLog.Error("failed-to-transition", err)
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
			vLog.Info("baggageclaim-url-is-missing")
			continue
		}

		baggageclaimClient := vc.baggageclaimClientFactory.NewClient(*destroyingVolume.Worker().BaggageclaimURL, destroyingVolume.Worker().Name)

		volume, found, err := baggageclaimClient.LookupVolume(vc.logger, destroyingVolume.Handle())
		if err != nil {
			vLog.Error("failed-to-lookup-volume-in-baggageclaim", err)
			continue
		}

		if vc.destroyRealVolume(vLog.Session("in-worker"), volume, found) {
			vc.destroyDBVolume(vLog.Session("in-db"), destroyingVolume)
		}
	}

	return nil
}

func (vc *volumeCollector) destroyRealVolume(logger lager.Logger, volume baggageclaim.Volume, found bool) bool {
	if found {
		logger.Debug("destroying")

		err := volume.Destroy()
		if err != nil {
			logger.Error("failed-to-destroy", err)
			return false
		}

		logger.Debug("destroyed")
	} else {
		logger.Debug("already-removed")
	}

	return true
}

func (vc *volumeCollector) destroyDBVolume(logger lager.Logger, dbVolume dbng.DestroyingVolume) {
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
