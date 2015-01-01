package exec

import (
	garden "github.com/cloudfoundry-incubator/garden/api"

	"github.com/concourse/atc"
	"github.com/concourse/atc/exec/resource"
)

type gardenFactory struct {
	gardenClient    garden.Client
	resourceTracker resource.Tracker
}

func NewGardenFactory(
	gardenClient garden.Client,
	resourceTracker resource.Tracker,
) Factory {
	return &gardenFactory{
		gardenClient:    gardenClient,
		resourceTracker: resourceTracker,
	}
}

func (factory *gardenFactory) Get(config atc.ResourceConfig, params atc.Params, version atc.Version) Step {
	return resourceStep{
		Tracker: factory.resourceTracker,
		Type:    resource.ResourceType(config.Type),

		Action: func(r resource.Resource, s ArtifactSource) resource.VersionedSource {
			return r.Get(config.Source, params, version)
		},
	}
}

func (factory *gardenFactory) Put(config atc.ResourceConfig, params atc.Params) Step {
	return resourceStep{
		Tracker: factory.resourceTracker,
		Type:    resource.ResourceType(config.Type),

		Action: func(r resource.Resource, s ArtifactSource) resource.VersionedSource {
			return r.Put(config.Source, params, resourceSource{s})
		},
	}
}

func (factory *gardenFactory) Execute(configSource BuildConfigSource) Step {
	return executeStep{
		GardenClient: factory.gardenClient,
		ConfigSource: configSource,
	}
}
