package resource

import (
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/worker"
)

type resourceInstanceFetchSource struct {
	logger                lager.Logger
	resourceInstance      ResourceInstance
	versionedSource       VersionedSource
	worker                worker.Worker
	resourceOptions       ResourceOptions
	resourceTypes         dbng.ResourceTypes
	tags                  atc.Tags
	teamID                int
	session               Session
	metadata              Metadata
	imageFetchingDelegate worker.ImageFetchingDelegate
}

func NewResourceInstanceFetchSource(
	logger lager.Logger,
	resourceInstance ResourceInstance,
	worker worker.Worker,
	resourceOptions ResourceOptions,
	resourceTypes dbng.ResourceTypes,
	tags atc.Tags,
	teamID int,
	session Session,
	metadata Metadata,
	imageFetchingDelegate worker.ImageFetchingDelegate,
) FetchSource {
	return &resourceInstanceFetchSource{
		logger:                logger,
		resourceInstance:      resourceInstance,
		worker:                worker,
		resourceOptions:       resourceOptions,
		resourceTypes:         resourceTypes,
		tags:                  tags,
		teamID:                teamID,
		session:               session,
		metadata:              metadata,
		imageFetchingDelegate: imageFetchingDelegate,
	}
}

func (s *resourceInstanceFetchSource) IsInitialized() (bool, error) {
	sLog := s.logger.Session("is-initialized")

	volume, found, err := s.resourceInstance.FindInitializedOn(s.logger, s.worker)
	if err != nil {
		sLog.Error("failed-to-find-initialized-on", err)
		return false, err
	}

	if found {
		s.versionedSource = NewGetVersionedSource(volume, s.resourceOptions.Version(), nil)
	}

	return found, nil
}

func (s *resourceInstanceFetchSource) VersionedSource() VersionedSource {
	return s.versionedSource
}

func (s *resourceInstanceFetchSource) LockName() (string, error) {
	return s.resourceOptions.LockName(s.worker.Name())
}

// Initialize runs under the lock but we need to make sure volume
// does not exist yet before creating it under the lock
func (s *resourceInstanceFetchSource) Initialize(signals <-chan os.Signal, ready chan<- struct{}) error {
	sLog := s.logger.Session("initialize")

	volume, found, err := s.resourceInstance.FindInitializedOn(s.logger, s.worker)
	if err != nil {
		sLog.Error("failed-to-find-initialized-on", err)
		return err
	}

	if found {
		sLog.Debug("already-initialized")
		s.versionedSource = NewGetVersionedSource(volume, s.resourceOptions.Version(), nil)
		return nil
	}

	volume, err = s.resourceInstance.CreateOn(sLog, s.worker)
	if err != nil {
		sLog.Error("failed-to-create-cache", err)
		return err
	}

	container, err := s.createContainerForVolume(volume)
	if err != nil {
		sLog.Error("failed-to-create-container", err)
		return err
	}

	s.versionedSource, err = NewResourceForContainer(container).Get(
		volume,
		s.resourceOptions.IOConfig(),
		s.resourceOptions.Source(),
		s.resourceOptions.Params(),
		s.resourceOptions.Version(),
		signals,
		ready,
	)
	if err == ErrAborted {
		sLog.Error("get-run-resource-aborted", err, lager.Data{"container": container.Handle()})
		return ErrInterrupted
	}

	if err != nil {
		sLog.Error("failed-to-fetch-resource", err)
		return err
	}

	err = volume.Initialize()
	if err != nil {
		sLog.Error("failed-to-initialize-cache", err)
		return err
	}

	return nil
}

func (s *resourceInstanceFetchSource) createContainerForVolume(volume worker.Volume) (worker.Container, error) {
	containerSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: string(s.resourceOptions.ResourceType()),
			Privileged:   true,
		},
		Ephemeral: s.session.Ephemeral,
		Tags:      s.tags,
		TeamID:    s.teamID,
		Env:       s.metadata.Env(),
		Mounts: []worker.VolumeMount{
			{
				Volume:    volume,
				MountPath: ResourcesDir("get"),
			},
		},
	}

	return s.worker.CreateResourceGetContainer(
		s.logger,
		s.resourceInstance.ResourceUser(),
		nil,
		s.imageFetchingDelegate,
		s.session.ID,
		s.session.Metadata,
		containerSpec,
		s.resourceTypes,
		map[string]string{},
		string(s.resourceOptions.ResourceType()),
		s.resourceOptions.Version(),
		s.resourceOptions.Source(),
		s.resourceOptions.Params(),
	)
}
