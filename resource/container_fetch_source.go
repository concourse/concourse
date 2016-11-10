package resource

import (
	"os"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/worker"
)

type containerFetchSource struct {
	logger          lager.Logger
	container       worker.Container
	volume          worker.Volume
	resourceOptions ResourceOptions
	versionedSource VersionedSource
}

func NewContainerFetchSource(
	logger lager.Logger,
	container worker.Container,
	volume worker.Volume,
	resourceOptions ResourceOptions,
) FetchSource {
	return &containerFetchSource{
		logger:          logger,
		container:       container,
		volume:          volume,
		versionedSource: NewGetVersionedSource(volume, resourceOptions.Version(), nil),
		resourceOptions: resourceOptions,
	}
}

func (s *containerFetchSource) IsInitialized() (bool, error) {
	return s.volume.IsInitialized()
}

func (s *containerFetchSource) VersionedSource() VersionedSource {
	return s.versionedSource
}

func (s *containerFetchSource) LockName() (string, error) {
	return s.resourceOptions.LockName(s.container.WorkerName())
}

func (s *containerFetchSource) Initialize(signals <-chan os.Signal, ready chan<- struct{}) error {
	var err error
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

func (s *containerFetchSource) Release(finalTTL *time.Duration) {
	s.container.Release(finalTTL)
}
