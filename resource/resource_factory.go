package resource

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
)

//go:generate counterfeiter . ResourceFactoryFactory

type ResourceFactoryFactory interface {
	FactoryFor(worker.Client) ResourceFactory
}

//go:generate counterfeiter . ResourceFactory

type ResourceFactory interface {
	NewBuildResource(
		logger lager.Logger,
		metadata Metadata,
		session Session,
		typ ResourceType,
		tags atc.Tags,
		teamID int,
		sources map[string]ArtifactSource,
		resourceTypes atc.ResourceTypes,
		imageFetchingDelegate worker.ImageFetchingDelegate,
	) (Resource, []string, error)

	NewCheckResource(
		logger lager.Logger,
		metadata Metadata,
		session Session,
		typ ResourceType,
		tags atc.Tags,
		teamID int,
		resourceTypes atc.ResourceTypes,
		imageFetchingDelegate worker.ImageFetchingDelegate,
	) (Resource, error)

	NewResourceTypeCheckResource(
		logger lager.Logger,
		metadata Metadata,
		session Session,
		typ ResourceType,
		tags atc.Tags,
		teamID int,
		resourceTypes atc.ResourceTypes,
		imageFetchingDelegate worker.ImageFetchingDelegate,
	) (Resource, error)
}

type resourceFactoryFactory struct {
}

func (f *resourceFactoryFactory) FactoryFor(client worker.Client) ResourceFactory {
	return NewResourceFactory(client)
}

func NewResourceFactoryFactory() ResourceFactoryFactory {
	return &resourceFactoryFactory{}
}

type resourceFactory struct {
	workerClient worker.Client
}

func NewResourceFactory(
	workerClient worker.Client,
) ResourceFactory {
	return &resourceFactory{
		workerClient: workerClient,
	}
}

func (f *resourceFactory) NewBuildResource(
	logger lager.Logger,
	metadata Metadata,
	session Session,
	typ ResourceType,
	tags atc.Tags,
	teamID int,
	sources map[string]ArtifactSource,
	resourceTypes atc.ResourceTypes,
	imageFetchingDelegate worker.ImageFetchingDelegate,
) (Resource, []string, error) {
	logger = logger.Session("new-put-resource")

	logger.Debug("start")
	defer logger.Debug("done")

	container, found, err := f.workerClient.FindContainerForIdentifier(logger, session.ID)
	if err != nil {
		logger.Error("failed-to-look-for-existing-container", err, lager.Data{"id": session.ID})
		return nil, nil, err
	}

	if found {
		logger.Debug("found-existing-container", lager.Data{"container": container.Handle()})

		return NewResource(container), []string{}, nil
	}

	resourceSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: string(typ),
			Privileged:   true,
		},
		Ephemeral: session.Ephemeral,
		Tags:      tags,
		TeamID:    teamID,
		Env:       metadata.Env(),
	}

	logger.Debug("finding-compatible-worker", lager.Data{"resourceSpec": resourceSpec, "resourceTypes": resourceTypes, "sources": sources})

	chosenWorker, mounts, missingSources, err := f.findCompatibleWorker(resourceSpec, resourceTypes, sources)
	if err != nil {
		if err == worker.ErrNotImplemented { // TODO: fix this
			chosenWorker = f.workerClient
		} else {
			logger.Error("failed-to-choose-worker", err)
			return nil, nil, err
		}
	}

	// logger.Debug("creating-container-supremo", lager.Data{"container-id": session.ID, "chosenWorker": chosenWorker, "mounts": mounts, "missingSources": missingSources})

	resourceSpec.Inputs = mounts
	container, err = chosenWorker.CreateBuildContainer(
		logger,
		nil,
		imageFetchingDelegate,
		session.ID,
		session.Metadata,
		resourceSpec,
		resourceTypes,
		map[string]string{},
	)
	if err != nil {
		logger.Error("failed-to-create-container", err)
		return nil, nil, err
	}

	logger.Info("created", lager.Data{"container": container.Handle()})

	return NewResource(container), missingSources, nil
}

func (f *resourceFactory) NewCheckResource(
	logger lager.Logger,
	metadata Metadata,
	session Session,
	typ ResourceType,
	tags atc.Tags,
	teamID int,
	resourceTypes atc.ResourceTypes,
	imageFetchingDelegate worker.ImageFetchingDelegate,
) (Resource, error) {
	logger = logger.Session("new-check-resource")

	logger.Debug("start")
	defer logger.Debug("done")

	container, found, err := f.workerClient.FindContainerForIdentifier(logger, session.ID)
	if err != nil {
		logger.Error("failed-to-look-for-existing-container", err, lager.Data{"id": session.ID})
		return nil, err
	}

	if found {
		logger.Debug("found-existing-container", lager.Data{"container": container.Handle()})
		return NewResource(container), nil
	}

	resourceSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: string(typ),
			Privileged:   true,
		},
		Ephemeral: session.Ephemeral,
		Tags:      tags,
		TeamID:    teamID,
		Env:       metadata.Env(),
	}

	logger.Debug("creating-container", lager.Data{"container-id": session.ID})

	container, err = f.workerClient.CreateResourceCheckContainer(
		logger,
		nil,
		imageFetchingDelegate,
		session.ID,
		session.Metadata,
		resourceSpec,
		resourceTypes,
		string(typ),
		session.ID.CheckSource,
	)
	if err != nil {
		logger.Error("failed-to-create-container", err)
		return nil, err
	}

	logger.Info("created", lager.Data{"container": container.Handle()})

	return NewResource(container), nil
}

func (f *resourceFactory) NewResourceTypeCheckResource(
	logger lager.Logger,
	metadata Metadata,
	session Session,
	typ ResourceType,
	tags atc.Tags,
	teamID int,
	resourceTypes atc.ResourceTypes,
	imageFetchingDelegate worker.ImageFetchingDelegate,
) (Resource, error) {
	logger = logger.Session("new-resource-type-check-resource")

	logger.Debug("start")
	defer logger.Debug("done")

	container, found, err := f.workerClient.FindContainerForIdentifier(logger, session.ID)
	if err != nil {
		logger.Error("failed-to-look-for-existing-container", err, lager.Data{"id": session.ID})
		return nil, err
	}

	if found {
		logger.Debug("found-existing-container", lager.Data{"container": container.Handle()})
		return NewResource(container), nil
	}

	resourceSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: string(typ),
			Privileged:   true,
		},
		Ephemeral: session.Ephemeral,
		Tags:      tags,
		TeamID:    teamID,
		Env:       metadata.Env(),
	}

	logger.Debug("creating-container", lager.Data{"container-id": session.ID})

	container, err = f.workerClient.CreateResourceTypeCheckContainer(
		logger,
		nil,
		imageFetchingDelegate,
		session.ID,
		session.Metadata,
		resourceSpec,
		resourceTypes,
		string(typ),
		session.ID.CheckSource,
	)
	if err != nil {
		logger.Error("failed-to-create-container", err)
		return nil, err
	}

	logger.Info("created", lager.Data{"container": container.Handle()})

	return NewResource(container), nil
}

func (f *resourceFactory) findCompatibleWorker(
	resourceSpec worker.ContainerSpec,
	resourceTypes atc.ResourceTypes,
	sources map[string]ArtifactSource,
) (worker.Client, []worker.VolumeMount, []string, error) {
	compatibleWorkers, err := f.workerClient.AllSatisfying(resourceSpec.WorkerSpec(), resourceTypes)
	if err != nil {
		return nil, nil, nil, err
	}

	// find the worker with the most volumes
	mounts := []worker.VolumeMount{}
	missingSources := []string{}
	var chosenWorker worker.Worker

	// for each worker that matches tags, platform, etc -- what is the etc?
	for _, w := range compatibleWorkers {
		candidateMounts := []worker.VolumeMount{}
		missing := []string{}

		for name, source := range sources {
			// look at all the inputs/outputs we're looking for
			ourVolume, found, err := source.VolumeOn(w)
			if err != nil {
				return nil, nil, nil, err
			}

			if found {
				candidateMounts = append(candidateMounts, worker.VolumeMount{
					Volume:    ourVolume,
					MountPath: ResourcesDir("put/" + name),
				})
			} else {
				missing = append(missing, name)
			}
		}

		if len(candidateMounts) >= len(mounts) {
			mounts = candidateMounts
			missingSources = missing
			chosenWorker = w
		}
	}

	return chosenWorker, mounts, missingSources, nil
}
