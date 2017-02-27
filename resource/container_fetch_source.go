package resource

import (
	"os"

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

// Initialize runs under the lock but we need to make sure volume
// does not exist yet before creating it under the lock
func (s *containerFetchSource) Initialize(signals <-chan os.Signal, ready chan<- struct{}) error {
	sLog := s.logger.Session("initialize")

	initialized, err := s.volume.IsInitialized()
	if err != nil {
		sLog.Error("failed-to-check-if-initialized", err)
		return err
	}

	if initialized {
		sLog.Debug("already-initialized")
		return nil
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
		sLog.Error("get-run-resource-aborted", err, lager.Data{"container": s.container.Handle()})
		return ErrInterrupted
	}

	if err != nil {
		sLog.Error("failed-to-fetch-resource", err)
		return err
	}

	err = s.volume.Initialize()
	if err != nil {
		sLog.Error("failed-to-initialize-cache", err)
		return err
	}

	return nil
}
