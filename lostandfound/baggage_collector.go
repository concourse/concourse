package lostandfound

import (
	"encoding/json"
	"time"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"

	"github.com/pivotal-golang/lager"
)

const NoRelevantVersionsTTL = 10 * time.Minute

type BaggageCollectorDB interface {
	GetAllActivePipelines() ([]db.SavedPipeline, error)
	GetVolumes() ([]db.SavedVolumeData, error)
	SetVolumeTTL(db.SavedVolumeData, time.Duration) error
}

//go:generate counterfeiter . BaggageCollector

type BaggageCollector interface {
	Collect() error
}

type baggageCollector struct {
	logger            lager.Logger
	workerClient      worker.Client
	db                BaggageCollectorDB
	pipelineDBFactory db.PipelineDBFactory
}

func (bc *baggageCollector) Collect() error {
	bc.logger.Info("collect")

	resourceHashVersions, err := bc.getResourceHashVersions()
	if err != nil {
		panic(err)
		return err
	}

	err = bc.expireVolumes(resourceHashVersions)
	if err != nil {
		panic(err)
		return err
	}
	return nil
}

type resourceHashVersion map[string]int

func (bc *baggageCollector) getResourceHashVersions() (resourceHashVersion, error) {
	bc.logger.Session("ranking-resource-versions")
	resourceHash := resourceHashVersion{}

	pipelines, err := bc.db.GetAllActivePipelines()
	if err != nil {
		bc.logger.Info("could-not-get-active-pipelines", lager.Data{"error": err.Error()})
		return nil, err
	}

	for _, pipeline := range pipelines {
		pipelineDB := bc.pipelineDBFactory.Build(pipeline)
		pipelineResources := pipeline.Config.Resources

		for _, pipelineResource := range pipelineResources {
			dbResource, err := pipelineDB.GetResource(pipelineResource.Name)
			if err != nil {
				return nil, err
			}
			maxID, err := pipelineDB.GetResourceHistoryMaxID(dbResource.ID)
			if err != nil {
				return nil, err
			}

			pipelineResourceVersions, _, err := pipelineDB.GetResourceHistoryCursor(pipelineResource.Name, maxID, false, 5)
			if err != nil {
				return nil, err
			}

			versionRank := 0
			for _, pipelineResourceVersion := range pipelineResourceVersions {
				if pipelineResourceVersion.VersionedResource.Enabled {

					version, _ := json.Marshal(pipelineResourceVersion.VersionedResource.Version)
					hashKey := string(version) + resource.GenerateResourceHash(pipelineResource.Source, pipelineResource.Type)

					if rank, ok := resourceHash[hashKey]; ok {
						resourceHash[hashKey] = min(rank, versionRank)
					} else {
						resourceHash[hashKey] = versionRank
					}

					versionRank++
				}
			}
		}
	}

	return resourceHash, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (bc *baggageCollector) expireVolumes(resourceHashVersions resourceHashVersion) error {
	volumesToExpire, err := bc.db.GetVolumes()
	rankToTTL := map[int]time.Duration{
		0: 24 * time.Hour,
		1: 8 * time.Hour,
		2: 4 * time.Hour,
		3: 2 * time.Hour,
		4: 1 * time.Hour,
	}

	if err != nil {
		bc.logger.Info("could-not-get-volume-data", lager.Data{"error": err.Error()})
		return err
	}

	for _, volumeToExpire := range volumesToExpire {
		version, _ := json.Marshal(volumeToExpire.ResourceVersion)
		hashKey := string(version) + volumeToExpire.ResourceHash
		ttlForVol := NoRelevantVersionsTTL

		if rank, ok := resourceHashVersions[hashKey]; ok {
			if rankToTTL[rank] == volumeToExpire.TTL {
				continue
			} else {
				ttlForVol = rankToTTL[rank]
			}
		}

		worker, err := bc.workerClient.GetWorker(volumeToExpire.WorkerName)
		if err != nil {
			bc.logger.Info("could-not-locate-worker", lager.Data{"error": err.Error()})
			continue
		}

		baggageClaimClient, found := worker.VolumeManager()

		if !found {
			bc.logger.Info("no-volume-manager-on-worker", lager.Data{"error": err.Error()})
			continue
		}

		volume, err := baggageClaimClient.LookupVolume(bc.logger, volumeToExpire.Handle)
		if err != nil {
			bc.logger.Info("could-not-locate-volume", lager.Data{"error": err.Error()})
			continue
		}

		volume.Release(ttlForVol)

		bc.db.SetVolumeTTL(volumeToExpire, ttlForVol)
		if err != nil {
			bc.logger.Info("failed-to-update-tll-in-db", lager.Data{"error": err.Error()})
		}
	}

	return nil

}

func NewBaggageCollector(
	logger lager.Logger,
	workerClient worker.Client,
	db BaggageCollectorDB,
	pipelineDBFactory db.PipelineDBFactory,
) BaggageCollector {
	return &baggageCollector{
		logger:            logger,
		workerClient:      workerClient,
		db:                db,
		pipelineDBFactory: pipelineDBFactory,
	}
}
