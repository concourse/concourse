package resource

import (
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/worker"
)

type resourceInstanceFetchSource struct {
	logger                 lager.Logger
	resourceCache          *dbng.UsedResourceCache
	resourceInstance       ResourceInstance
	worker                 worker.Worker
	resourceOptions        ResourceOptions
	resourceTypes          atc.VersionedResourceTypes
	tags                   atc.Tags
	teamID                 int
	session                Session
	metadata               Metadata
	imageFetchingDelegate  worker.ImageFetchingDelegate
	dbResourceCacheFactory dbng.ResourceCacheFactory
}

func NewResourceInstanceFetchSource(
	logger lager.Logger,
	resourceCache *dbng.UsedResourceCache,
	resourceInstance ResourceInstance,
	worker worker.Worker,
	resourceOptions ResourceOptions,
	resourceTypes atc.VersionedResourceTypes,
	tags atc.Tags,
	teamID int,
	session Session,
	metadata Metadata,
	imageFetchingDelegate worker.ImageFetchingDelegate,
	dbResourceCacheFactory dbng.ResourceCacheFactory,
) FetchSource {
	return &resourceInstanceFetchSource{
		logger:                 logger,
		resourceCache:          resourceCache,
		resourceInstance:       resourceInstance,
		worker:                 worker,
		resourceOptions:        resourceOptions,
		resourceTypes:          resourceTypes,
		tags:                   tags,
		teamID:                 teamID,
		session:                session,
		metadata:               metadata,
		imageFetchingDelegate:  imageFetchingDelegate,
		dbResourceCacheFactory: dbResourceCacheFactory,
	}
}

func (s *resourceInstanceFetchSource) LockName() (string, error) {
	return s.resourceOptions.LockName(s.worker.Name())
}

func (s *resourceInstanceFetchSource) FindInitialized() (VersionedSource, bool, error) {
	sLog := s.logger.Session("is-initialized")

	volume, found, err := s.resourceInstance.FindInitializedOn(s.logger, s.worker)
	if err != nil {
		sLog.Error("failed-to-find-initialized-on", err)
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	s.logger.Debug("found-initialized-versioned-source", lager.Data{"version": s.resourceOptions.Version(), "metadata": s.resourceCache.Metadata.ToATCMetadata()})

	return NewGetVersionedSource(
		volume,
		s.resourceOptions.Version(),
		s.resourceCache.Metadata.ToATCMetadata(),
	), true, nil
}

// Initialize runs under the lock but we need to make sure volume
// does not exist yet before creating it under the lock
func (s *resourceInstanceFetchSource) Initialize(signals <-chan os.Signal, ready chan<- struct{}) (VersionedSource, error) {
	sLog := s.logger.Session("initialize")

	versionedSource, found, err := s.FindInitialized()
	if err != nil {
		return nil, err
	}

	if found {
		return versionedSource, nil
	}

	volume, err := s.resourceInstance.CreateOn(sLog, s.worker)
	if err != nil {
		sLog.Error("failed-to-create-cache", err)
		return nil, err
	}

	container, err := s.createContainerForVolume(volume)
	if err != nil {
		sLog.Error("failed-to-create-container", err)
		return nil, err
	}

	versionedSource, err = NewResourceForContainer(container).Get(
		volume,
		s.resourceOptions.IOConfig(),
		s.resourceOptions.Source(),
		s.resourceOptions.Params(),
		s.resourceOptions.Version(),
		signals,
		ready,
	)
	if err != nil {
		if err == ErrAborted {
			sLog.Error("get-run-resource-aborted", err, lager.Data{"container": container.Handle()})
			return nil, ErrInterrupted
		}

		sLog.Error("failed-to-fetch-resource", err)
		return nil, err
	}

	err = volume.Initialize()
	if err != nil {
		sLog.Error("failed-to-initialize-cache", err)
		return nil, err
	}

	err = s.dbResourceCacheFactory.UpdateResourceCacheMetadata(s.resourceCache, versionedSource.Metadata())
	if err != nil {
		s.logger.Error("failed-to-update-resource-cache-metadata", err, lager.Data{"resource-cache": s.resourceCache})
		return nil, err
	}

	return versionedSource, nil
}

func (s *resourceInstanceFetchSource) createContainerForVolume(volume worker.Volume) (worker.Container, error) {
	containerSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: string(s.resourceOptions.ResourceType()),
			Privileged:   true,
		},
		Tags:   s.tags,
		TeamID: s.teamID,
		Env:    s.metadata.Env(),

		ResourceCache: &worker.VolumeMount{
			Volume:    volume,
			MountPath: ResourcesDir("get"),
		},
	}

	return s.worker.CreateResourceGetContainer(
		s.logger,
		s.resourceInstance.ResourceUser(),
		nil,
		s.imageFetchingDelegate,
		s.session.Metadata,
		containerSpec,
		s.resourceTypes,
		string(s.resourceOptions.ResourceType()),
		s.resourceOptions.Version(),
		s.resourceOptions.Source(),
		s.resourceOptions.Params(),
	)
}
