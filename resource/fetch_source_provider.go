package resource

import (
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
)

//go:generate counterfeiter . FetchSourceProviderFactory

type FetchSourceProviderFactory interface {
	NewFetchSourceProvider(
		logger lager.Logger,
		session Session,
		metadata Metadata,
		tags atc.Tags,
		teamID int,
		resourceTypes atc.VersionedResourceTypes,
		resourceInstance ResourceInstance,
		resourceOptions ResourceOptions,
		imageFetchingDelegate worker.ImageFetchingDelegate,
	) FetchSourceProvider
}

//go:generate counterfeiter . FetchSourceProvider

type FetchSourceProvider interface {
	Get() (FetchSource, error)
}

//go:generate counterfeiter . FetchSource

type FetchSource interface {
	IsInitialized() (bool, error)
	LockName() (string, error)
	VersionedSource() VersionedSource
	Initialize(signals <-chan os.Signal, ready chan<- struct{}) error
}

type fetchSourceProviderFactory struct {
	workerClient worker.Client
}

func NewFetchSourceProviderFactory(workerClient worker.Client) FetchSourceProviderFactory {
	return &fetchSourceProviderFactory{
		workerClient: workerClient,
	}
}

func (f *fetchSourceProviderFactory) NewFetchSourceProvider(
	logger lager.Logger,
	session Session,
	metadata Metadata,
	tags atc.Tags,
	teamID int,
	resourceTypes atc.VersionedResourceTypes,
	resourceInstance ResourceInstance,
	resourceOptions ResourceOptions,
	imageFetchingDelegate worker.ImageFetchingDelegate,
) FetchSourceProvider {
	return &fetchSourceProvider{
		logger:                logger,
		session:               session,
		metadata:              metadata,
		tags:                  tags,
		teamID:                teamID,
		resourceTypes:         resourceTypes,
		resourceInstance:      resourceInstance,
		resourceOptions:       resourceOptions,
		imageFetchingDelegate: imageFetchingDelegate,
		workerClient:          f.workerClient,
	}
}

type fetchSourceProvider struct {
	logger                lager.Logger
	session               Session
	metadata              Metadata
	tags                  atc.Tags
	teamID                int
	resourceTypes         atc.VersionedResourceTypes
	resourceInstance      ResourceInstance
	resourceOptions       ResourceOptions
	workerClient          worker.Client
	imageFetchingDelegate worker.ImageFetchingDelegate
}

func (f *fetchSourceProvider) Get() (FetchSource, error) {
	resourceSpec := worker.WorkerSpec{
		ResourceType: string(f.resourceOptions.ResourceType()),
		Tags:         f.tags,
		TeamID:       f.teamID,
	}

	chosenWorker, err := f.workerClient.Satisfying(resourceSpec, f.resourceTypes)
	if err != nil {
		f.logger.Error("no-workers-satisfying-spec", err)
		return nil, err
	}

	return NewResourceInstanceFetchSource(
		f.logger,
		f.resourceInstance,
		chosenWorker,
		f.resourceOptions,
		f.resourceTypes,
		f.tags,
		f.teamID,
		f.session,
		f.metadata,
		f.imageFetchingDelegate,
	), nil
}

func findCacheVolumeForContainer(container worker.Container) (worker.Volume, bool) {
	for _, mount := range container.VolumeMounts() {
		if mount.MountPath == ResourcesDir("get") {
			return mount.Volume, true
		}
	}

	return nil, false
}
