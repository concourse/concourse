package lostandfound

import (
	"encoding/json"
	"time"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"

	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . BaggageCollectorDB

type BaggageCollectorDB interface {
	ReapVolume(string) error
	GetAllActivePipelines() ([]db.SavedPipeline, error)
	GetVolumes() ([]db.SavedVolume, error)
	SetVolumeTTL(string, time.Duration) error
}

//go:generate counterfeiter . BaggageCollector

type BaggageCollector interface {
	Collect() error
}

type baggageCollector struct {
	logger                 lager.Logger
	workerClient           worker.Client
	db                     BaggageCollectorDB
	pipelineDBFactory      db.PipelineDBFactory
	oldResourceGracePeriod time.Duration
}

func (bc *baggageCollector) Collect() error {
	bc.logger.Info("collect")

	latestVersions, err := bc.getLatestVersionSet()
	if err != nil {
		return err
	}

	err = bc.expireVolumes(latestVersions)
	if err != nil {
		return err
	}
	return nil
}

type hashedVersionSet map[string]bool

func (bc *baggageCollector) getLatestVersionSet() (hashedVersionSet, error) {
	latestVersions := hashedVersionSet{}

	pipelines, err := bc.db.GetAllActivePipelines()
	if err != nil {
		bc.logger.Error("could-not-get-active-pipelines", err)
		return nil, err
	}

	for _, pipeline := range pipelines {
		pipelineDB := bc.pipelineDBFactory.Build(pipeline)
		pipelineResources := pipeline.Config.Resources

		for _, pipelineResource := range pipelineResources {
			logger := bc.logger.WithData(lager.Data{
				"pipeline": pipeline.Name,
				"resource": pipelineResource.Name,
			})

			latestEnabledVersion, found, err := pipelineDB.GetLatestEnabledVersionedResource(pipelineResource.Name)
			if err != nil {
				logger.Error("could-not-get-latest-enabled-resource", err)
				return nil, err
			}

			if !found {
				continue
			}

			version, _ := json.Marshal(latestEnabledVersion.VersionedResource.Version)
			hashKey := string(version) + resource.GenerateResourceHash(
				pipelineResource.Source, pipelineResource.Type,
			)
			latestVersions[hashKey] = true
		}
	}

	return latestVersions, nil
}

func (bc *baggageCollector) expireVolumes(latestVersions hashedVersionSet) error {
	volumesToExpire, err := bc.db.GetVolumes()

	if err != nil {
		bc.logger.Error("could-not-get-volume-data", err)
		return err
	}

	for _, volumeToExpire := range volumesToExpire {
		version, _ := json.Marshal(volumeToExpire.ResourceVersion)
		hashKey := string(version) + volumeToExpire.ResourceHash

		ttlForVol := bc.oldResourceGracePeriod
		if latestVersions[hashKey] {
			ttlForVol = 0 // live forever
		}

		volumeWorker, err := bc.workerClient.GetWorker(volumeToExpire.WorkerID)
		if err != nil {
			bc.logger.Info("could-not-locate-worker", lager.Data{
				"error":     err.Error(),
				"worker-id": volumeToExpire.WorkerID,
			})
			bc.db.ReapVolume(volumeToExpire.Handle)
			continue
		}

		if volumeToExpire.TTL == ttlForVol {
			continue
		}

		baggageClaimClient, found := volumeWorker.VolumeManager()
		if !found {
			bc.logger.Info("no-volume-manager-on-worker", lager.Data{
				"worker-id": volumeToExpire.WorkerID,
			})
			bc.db.ReapVolume(volumeToExpire.Handle)
			continue
		}

		volume, found, err := baggageClaimClient.LookupVolume(bc.logger, volumeToExpire.Handle)
		if !found || err != nil {
			var e string
			if err != nil {
				e = err.Error()
			}
			bc.logger.Info("could-not-locate-volume", lager.Data{
				"error":     e,
				"worker-id": volumeToExpire.WorkerID,
				"handle":    volumeToExpire.Handle,
			})
			continue
		}

		volume.Release(ttlForVol)
		err = bc.db.SetVolumeTTL(volumeToExpire.Handle, ttlForVol)
		if err != nil {
			bc.logger.Error("failed-to-update-ttl-in-db", err)
		}
	}

	return nil
}

func NewBaggageCollector(
	logger lager.Logger,
	workerClient worker.Client,
	db BaggageCollectorDB,
	pipelineDBFactory db.PipelineDBFactory,
	oldResourceGracePeriod time.Duration,
) BaggageCollector {
	return &baggageCollector{
		logger:                 logger,
		workerClient:           workerClient,
		db:                     db,
		pipelineDBFactory:      pipelineDBFactory,
		oldResourceGracePeriod: oldResourceGracePeriod,
	}
}
