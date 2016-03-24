package lostandfound

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"

	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . BaggageCollectorDB

type BaggageCollectorDB interface {
	ReapVolume(string) error
	GetAllPipelines() ([]db.SavedPipeline, error)
	GetVolumes() ([]db.SavedVolume, error)
	GetImageResourceCacheIdentifiersByBuildID(buildID int) ([]db.ResourceCacheIdentifier, error)
	GetVolumesForOneOffBuildImageResources() ([]db.SavedVolume, error)
}

//go:generate counterfeiter . BaggageCollector

type BaggageCollector interface {
	Collect() error
}

type baggageCollector struct {
	logger                              lager.Logger
	workerClient                        worker.Client
	db                                  BaggageCollectorDB
	pipelineDBFactory                   db.PipelineDBFactory
	oldResourceGracePeriod              time.Duration
	oneOffBuildImageResourceGracePeriod time.Duration
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

type hashedVersionSet map[string]time.Duration

func insertOrIncreaseVersionTTL(hvs hashedVersionSet, key string, ttl time.Duration) {
	oldTTL, found := hvs[key]
	if !found || ttlGreater(ttl, oldTTL) {
		hvs[key] = ttl
	}
}

func ttlGreater(ttl time.Duration, oldTTL time.Duration) bool {
	if oldTTL == 0 {
		return false
	}
	if ttl == 0 {
		return true
	}
	return ttl > oldTTL
}

func (bc *baggageCollector) getLatestVersionSet() (hashedVersionSet, error) {
	latestVersions := hashedVersionSet{}

	pipelines, err := bc.db.GetAllPipelines()
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

			insertOrIncreaseVersionTTL(latestVersions, hashKey, 0) // live forever
		}

		for _, pipelineJob := range pipeline.Config.Jobs {
			logger := bc.logger.WithData(lager.Data{
				"pipeline": pipeline.Name,
				"job":      pipelineJob.Name,
			})

			finished, _, err := pipelineDB.GetJobFinishedAndNextBuild(pipelineJob.Name)
			if err != nil {
				logger.Error("could-not-acquire-finished-and-next-builds-for-job", err)
				return nil, err
			}

			if finished != nil {
				resourceCacheIdentifiers, err := bc.db.GetImageResourceCacheIdentifiersByBuildID(finished.ID)
				if err != nil {
					logger.Error("could-not-acquire-volume-identifiers-for-build", err)
					return nil, err
				}

				for _, identifier := range resourceCacheIdentifiers {
					version, _ := json.Marshal(identifier.ResourceVersion)
					hashKey := string(version) + identifier.ResourceHash
					insertOrIncreaseVersionTTL(latestVersions, hashKey, 0) // live forever
				}
			}
		}
	}

	logger := bc.logger.Session("image-resources-for-one-off-builds")
	oneOffImageResourceVolumes, err := bc.db.GetVolumesForOneOffBuildImageResources()
	if err != nil {
		logger.Error("could-not-get-volumes-for-one-off-build-image-resources", err)
		return nil, err
	}

	for _, savedVolume := range oneOffImageResourceVolumes {
		hashKey, isResourceCache := resourceCacheHashKey(savedVolume)
		if !isResourceCache {
			continue
		}

		insertOrIncreaseVersionTTL(latestVersions, hashKey, bc.oneOffBuildImageResourceGracePeriod)
	}

	return latestVersions, nil
}

func (bc *baggageCollector) getSortedVolumes() ([]db.SavedVolume, error) {
	unsorted, err := bc.db.GetVolumes()
	if err != nil {
		return []db.SavedVolume{}, err
	}

	sort.Sort(sortByHandle(unsorted))

	sorted := unsorted

	return sorted, nil
}

type resourceCacheIdentifierAndWorkerNameSet map[string]bool

func resourceCacheHashKey(volume db.SavedVolume) (string, bool) {
	resourceCacheID := volume.Volume.Identifier.ResourceCache
	if resourceCacheID == nil {
		return "", false
	}

	version, _ := json.Marshal(resourceCacheID.ResourceVersion)
	return string(version) + resourceCacheID.ResourceHash, true
}

func (bc *baggageCollector) expireVolumes(latestVersions hashedVersionSet) error {
	volumesToExpire, err := bc.getSortedVolumes()

	seenResourceCacheIdentifierAndWorkerNameCombos := resourceCacheIdentifierAndWorkerNameSet{}

	if err != nil {
		bc.logger.Error("could-not-get-volume-data", err)
		return err
	}

	for _, volumeToExpire := range volumesToExpire {
		hashKey, isResourceCache := resourceCacheHashKey(volumeToExpire)
		if !isResourceCache {
			continue
		}

		var ttlForVol time.Duration

		resourceCacheIdentifierAndWorkerNameCombo := hashKey + volumeToExpire.WorkerName
		if contains(seenResourceCacheIdentifierAndWorkerNameCombos, resourceCacheIdentifierAndWorkerNameCombo) {
			ttlForVol = bc.oldResourceGracePeriod
		} else {
			seenResourceCacheIdentifierAndWorkerNameCombos[resourceCacheIdentifierAndWorkerNameCombo] = true
			ttlForVol = getWithDefault(latestVersions, hashKey, bc.oldResourceGracePeriod)
		}

		volumeWorker, err := bc.workerClient.GetWorker(volumeToExpire.WorkerName)
		if err != nil {
			bc.logger.Info("could-not-locate-worker", lager.Data{
				"error":     err.Error(),
				"worker-id": volumeToExpire.WorkerName,
			})
			bc.db.ReapVolume(volumeToExpire.Handle)
			continue
		}

		if volumeToExpire.TTL == ttlForVol {
			continue
		}

		vLogger := bc.logger.Session("volume", lager.Data{
			"worker-name": volumeToExpire.WorkerName,
			"handle":      volumeToExpire.Handle,
		})

		volume, found, err := volumeWorker.LookupVolume(bc.logger, volumeToExpire.Handle)
		if err != nil {
			vLogger.Error("failed-to-lookup-volume", err)
			continue
		}

		if !found {
			vLogger.Info("volume-not-found")
			bc.db.ReapVolume(volumeToExpire.Handle)
			continue
		}

		vLogger.Debug("releasing", lager.Data{
			"ttl": ttlForVol,
		})

		volume.Release(worker.FinalTTL(ttlForVol))
	}

	return nil
}

func NewBaggageCollector(
	logger lager.Logger,
	workerClient worker.Client,
	db BaggageCollectorDB,
	pipelineDBFactory db.PipelineDBFactory,
	oldResourceGracePeriod time.Duration,
	oneOffBuildImageResourceGracePeriod time.Duration,
) BaggageCollector {
	return &baggageCollector{
		logger:                              logger,
		workerClient:                        workerClient,
		db:                                  db,
		pipelineDBFactory:                   pipelineDBFactory,
		oldResourceGracePeriod:              oldResourceGracePeriod,
		oneOffBuildImageResourceGracePeriod: oneOffBuildImageResourceGracePeriod,
	}
}

type sortByHandle []db.SavedVolume

func (s sortByHandle) Len() int           { return len(s) }
func (s sortByHandle) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s sortByHandle) Less(i, j int) bool { return s[i].Volume.Handle < s[j].Volume.Handle }

func contains(set resourceCacheIdentifierAndWorkerNameSet, item string) bool {
	value, found := set[item]
	return found && value
}

func getWithDefault(latestVersions hashedVersionSet, hashKey string, defaultTTL time.Duration) time.Duration {
	ttl, found := latestVersions[hashKey]
	if !found {
		ttl = defaultTTL
	}
	return ttl
}
