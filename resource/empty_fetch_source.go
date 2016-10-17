package resource

import (
	"os"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/worker"
)

type emptyFetchSource struct {
	logger           lager.Logger
	worker           worker.Worker
	cache            Cache
	container        worker.Container
	versionedSource  VersionedSource
	cacheIdentifier  CacheIdentifier
	containerCreator FetchContainerCreator
	resourceOptions  ResourceOptions
}

func NewEmptyFetchSource(
	logger lager.Logger,
	worker worker.Worker,
	cacheIdentifier CacheIdentifier,
	containerCreator FetchContainerCreator,
	resourceOptions ResourceOptions,
) FetchSource {
	return &emptyFetchSource{
		logger:           logger,
		worker:           worker,
		cache:            noopCache{},
		cacheIdentifier:  cacheIdentifier,
		containerCreator: containerCreator,
		resourceOptions:  resourceOptions,
	}
}

func (s *emptyFetchSource) IsInitialized() (bool, error) {
	return s.cache.IsInitialized()
}

func (s *emptyFetchSource) VersionedSource() VersionedSource {
	return s.versionedSource
}

func (s *emptyFetchSource) LockName() (string, error) {
	return s.resourceOptions.LockName(s.worker.Name())
}

func (s *emptyFetchSource) Initialize(signals <-chan os.Signal, ready chan<- struct{}) error {
	var err error
	s.cache, err = s.findOrCreateCacheVolume()
	if err != nil {
		return err
	}

	s.logger.Debug("creating container with volume", lager.Data{
		"volume-handle": s.cache.Volume().Handle(),
		"volume-path":   s.cache.Volume().Path(),
	})
	s.container, err = s.containerCreator.CreateWithVolume(s.resourceOptions, s.cache.Volume(), s.worker)
	if err != nil {
		s.logger.Error("failed-to-create-container", err)
		return err
	}

	s.versionedSource, err = NewResource(s.container).Get(
		s.cache.Volume(),
		s.resourceOptions.IOConfig(),
		s.resourceOptions.Source(),
		s.resourceOptions.Params(),
		s.resourceOptions.Version(),
		signals,
		ready,
	)
	if err == ErrAborted {
		s.logger.Error("get-run-resource-aborted", err, lager.Data{"container": s.container.Handle()})
		return ErrInterrupted
	}

	if err != nil {
		s.logger.Error("failed-to-fetch-resource", err)
		return err
	}

	err = s.cache.Initialize()
	if err != nil {
		s.logger.Error("failed-to-initialize-cache", err)
		return err
	}

	return nil
}

func (s *emptyFetchSource) Release(finalTTL *time.Duration) {
	if s.container != nil {
		s.container.Release(finalTTL)
	}
}

func (s *emptyFetchSource) findOrCreateCacheVolume() (Cache, error) {
	cachedVolume, cacheFound, err := s.cacheIdentifier.FindOn(s.logger, s.worker)
	if err != nil {
		s.logger.Error("failed-to-look-for-cache", err)
		return nil, err
	}

	if cacheFound {
		s.logger.Debug("found-cache", lager.Data{"volume": cachedVolume.Handle()})
	} else {
		s.logger.Debug("no-cache-found")

		cachedVolume, err = s.cacheIdentifier.CreateOn(s.logger, s.worker)
		if err != nil {
			s.logger.Error("failed-to-create-cache", err)
			return nil, err
		}
	}

	return volumeCache{cachedVolume}, nil
}
