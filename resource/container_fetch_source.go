package resource

import (
	"os"
	"time"

	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/lager"
)

type containerFetchSource struct {
	logger          lager.Logger
	container       worker.Container
	versionedSource VersionedSource
	resource        Resource
	cache           Cache
	resourceOptions ResourceOptions
}

func NewContainerFetchSource(
	logger lager.Logger,
	container worker.Container,
	resourceOptions ResourceOptions,
) FetchSource {
	mounts := container.VolumeMounts()
	var cache Cache
	cache = noopCache{}

	for _, mount := range mounts {
		if mount.MountPath == ResourcesDir("get") {
			cache = volumeCache{mount.Volume}
		}
	}

	return &containerFetchSource{
		logger:          logger,
		container:       container,
		cache:           cache,
		resourceOptions: resourceOptions,
		versionedSource: NewGetVersionedSource(cache.Volume(), resourceOptions.Version(), nil),
	}
}

func (s *containerFetchSource) IsInitialized() (bool, error) {
	return s.cache.IsInitialized()
}

func (s *containerFetchSource) VersionedSource() VersionedSource {
	return s.versionedSource
}

func (s *containerFetchSource) LeaseName() (string, error) {
	return s.resourceOptions.LeaseName(s.container.WorkerName())
}

func (s *containerFetchSource) Initialize(signals <-chan os.Signal, ready chan<- struct{}) error {
	var err error
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

func (s *containerFetchSource) Release(finalTTL *time.Duration) {
	s.container.Release(finalTTL)
}
