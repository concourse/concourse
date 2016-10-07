package resource

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
)

//go:generate counterfeiter . ResourceFactory

type ResourceFactory interface {
	NewPutResource(
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

func (f *resourceFactory) NewPutResource(
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
	logger = logger.Session("[super-logs] new-put-resource")

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

	chosenWorker, mounts, missingSources, err := f.findCompatibleWorker(resourceSpec, resourceTypes, sources)

	logger.Debug("creating-container", lager.Data{"container-id": session.ID})

	resourceSpec.Inputs = mounts
	container, err = chosenWorker.CreateResourcePutContainer(
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

func (f *resourceFactory) findCompatibleWorker(
	resourceSpec worker.ContainerSpec,
	resourceTypes atc.ResourceTypes,
	sources map[string]ArtifactSource,
) (worker.Worker, []worker.VolumeMount, []string, error) {
	compatibleWorkers, err := f.workerClient.AllSatisfying(resourceSpec.WorkerSpec(), resourceTypes)
	if err != nil {
		return nil, nil, nil, err
	}

	// find the worker with the most volumes
	mounts := []worker.VolumeMount{}
	missingSources := []string{}
	var chosenWorker worker.Worker

	for _, w := range compatibleWorkers {
		candidateMounts := []worker.VolumeMount{}
		missing := []string{}

		for name, source := range sources {
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
			for _, mount := range mounts {
				mount.Volume.Release(nil)
			}

			mounts = candidateMounts
			missingSources = missing
			chosenWorker = w
		} else {
			for _, mount := range candidateMounts {
				mount.Volume.Release(nil)
			}
		}
	}

	return chosenWorker, mounts, missingSources, nil
}
