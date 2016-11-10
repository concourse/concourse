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
		resourceOptions:  resourceOptions,
		versionedSource:  NewGetVersionedSource(volume, resourceOptions.Version(), nil),
		containerCreator: containerCreator,
	}
}

func (s *volumeFetchSource) IsInitialized() (bool, error) {
	return s.volume.IsInitialized()
}

func (s *volumeFetchSource) VersionedSource() VersionedSource {
	return s.versionedSource
}

func (s *volumeFetchSource) LockName() (string, error) {
	return s.resourceOptions.LockName(s.worker.Name())
}

func (s *volumeFetchSource) Initialize(signals <-chan os.Signal, ready chan<- struct{}) error {
	var err error
	s.container, err = s.containerCreator.CreateWithVolume(s.resourceOptions, s.volume, s.worker)
	if err != nil {
		s.logger.Error("failed-to-create-container", err)
		return err
	}

	s.versionedSource, err = NewResourceForContainer(s.container).Get(
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

	err = s.volume.Initialize()
	if err != nil {
		s.logger.Error("failed-to-initialize-cache", err)
		return err
	}

	return nil
}

func (s *volumeFetchSource) Release(finalTTL *time.Duration) {
	if s.container != nil {
		s.container.Release(finalTTL)
	}
}
