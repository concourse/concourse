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
	GetAllActivePipelines() ([]db.SavedPipeline, error)
	GetVolumes() ([]db.SavedVolume, error)
	SetVolumeTTL(db.SavedVolume, time.Duration) error
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
	bc.logger.Session("ranking-resource-versions")
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
			pipelineResourceVersions, _, err := pipelineDB.GetResourceVersions(pipelineResource.Name, db.Page{Limit: 2})
			if err != nil {
				bc.logger.Error("could-not-get-resource-history", err)
				return nil, err
			}

			for _, pipelineResourceVersion := range pipelineResourceVersions {
				if pipelineResourceVersion.Enabled {
					version, _ := json.Marshal(pipelineResourceVersion.VersionedResource.Version)
					hashKey := string(version) + resource.GenerateResourceHash(pipelineResource.Source, pipelineResource.Type)
					latestVersions[hashKey] = true
					break
				}
			}
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

		if volumeToExpire.TTL == ttlForVol {
			continue
		}

		volumeWorker, err := bc.workerClient.GetWorker(volumeToExpire.WorkerName)
		if err != nil {
			bc.logger.Info("could-not-locate-worker", lager.Data{
				"error":  err.Error(),
				"worker": volumeToExpire.WorkerName,
			})
			continue
		}

		baggageClaimClient, found := volumeWorker.VolumeManager()

		if !found {
			bc.logger.Info("no-volume-manager-on-worker", lager.Data{
				"error":  err.Error(),
				"worker": volumeToExpire.WorkerName,
			})
			continue
		}

		volume, found, err := baggageClaimClient.LookupVolume(bc.logger, volumeToExpire.Handle)
		if !found || err != nil {
			var e string
			if err != nil {
				e = err.Error()
			}
			bc.logger.Info("could-not-locate-volume", lager.Data{
				"error":  e,
				"worker": volumeToExpire.WorkerName,
				"handle": volumeToExpire.Handle,
			})
			continue
		}

		volume.Release(ttlForVol)
		err = bc.db.SetVolumeTTL(volumeToExpire, ttlForVol)
		if err != nil {
			bc.logger.Error("failed-to-update-tll-in-db", err)
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
