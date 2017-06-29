package gc

import (
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/baggageclaim"
	bclient "github.com/concourse/baggageclaim/client"
)

type volumeCollector struct {
	rootLogger                lager.Logger
	volumeFactory             db.VolumeFactory
	workerFactory             db.WorkerFactory
	baggageclaimClientFactory BaggageclaimClientFactory
}

//go:generate counterfeiter . BaggageclaimClientFactory

type BaggageclaimClientFactory interface {
	NewClient(apiURL string, workerName string) bclient.Client
}

type baggageclaimClientFactory struct {
	dbWorkerFactory db.WorkerFactory
}

func NewBaggageclaimClientFactory(dbWorkerFactory db.WorkerFactory) BaggageclaimClientFactory {
	return &baggageclaimClientFactory{
		dbWorkerFactory: dbWorkerFactory,
	}
}

func (f *baggageclaimClientFactory) NewClient(apiURL string, workerName string) bclient.Client {
	return bclient.NewWithHTTPClient(apiURL, &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives:     true,
			ResponseHeaderTimeout: 1 * time.Minute,
		},
	})
}

func NewVolumeCollector(
	logger lager.Logger,
	volumeFactory db.VolumeFactory,
	workerFactory db.WorkerFactory,
	baggageclaimClientFactory BaggageclaimClientFactory,
) Collector {
	return &volumeCollector{
		rootLogger:                logger,
		volumeFactory:             volumeFactory,
		workerFactory:             workerFactory,
		baggageclaimClientFactory: baggageclaimClientFactory,
	}
}

func (vc *volumeCollector) Run() error {
	logger := vc.rootLogger.Session("run")

	logger.Debug("start")
	defer logger.Debug("done")

	workers, err := vc.workerFactory.Workers()
	if err != nil {
		logger.Error("failed-to-get-workers", err)
		return err
	}

	baggageClaimClients := map[string]bclient.Client{}
	for _, worker := range workers {
		if worker.BaggageclaimURL() != nil {
			baggageClaimClients[worker.Name()] = vc.baggageclaimClientFactory.NewClient(*worker.BaggageclaimURL(), worker.Name())
		}
	}

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

		baggageClaimClient, found := baggageClaimClients[destroyingVolume.WorkerName()]
		if !found {
			vLog.Info("baggageclaim-client-is-missing")
			continue
		}

		volume, found, err := baggageClaimClient.LookupVolume(vLog, destroyingVolume.Handle())
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

func (vc *volumeCollector) destroyDBVolume(logger lager.Logger, dbVolume db.DestroyingVolume) {
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
