package resource

import (
	"os"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/worker"
)

type volumeFetchSource struct {
	logger           lager.Logger
	volume           worker.Volume
	container        worker.Container
	cache            Cache
	versionedSource  VersionedSource
	worker           worker.Worker
	resourceOptions  ResourceOptions
	containerCreator FetchContainerCreator
}

func NewVolumeFetchSource(
	logger lager.Logger,
	volume worker.Volume,
	worker worker.Worker,
	resourceOptions ResourceOptions,
	containerCreator FetchContainerCreator,
) FetchSource {
	return &volumeFetchSource{
		logger:           logger,
		volume:           volume,
		worker:           worker,
		cache:            volumeCache{volume},
		resourceOptions:  resourceOptions,
		versionedSource:  NewGetVersionedSource(volume, resourceOptions.Version(), nil),
		containerCreator: containerCreator,
	}
}

func (s *volumeFetchSource) IsInitialized() (bool, error) {
	return s.cache.IsInitialized()
}

func (s *volumeFetchSource) VersionedSource() VersionedSource {
	return s.versionedSource
}

func (s *volumeFetchSource) LeaseName() (string, error) {
	return s.resourceOptions.LeaseName(s.worker.Name())
}

func (s *volumeFetchSource) Initialize(signals <-chan os.Signal, ready chan<- struct{}) error {
	var err error
	s.container, err = s.containerCreator.CreateWithVolume(string(s.resourceOptions.ResourceType()), s.volume, s.worker)
	if err != nil {
		s.logger.Error("failed-to-create-container", err)
		return err
	}

	s.versionedSource, err = NewResource(s.container).Get(
		s.volume,
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

func (s *volumeFetchSource) Release(finalTTL *time.Duration) {
	s.volume.Release(finalTTL)

	if s.container != nil {
		s.container.Release(finalTTL)
	}
}
