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

func (factory *gardenFactory) Get(ioConfig IOConfig, config atc.ResourceConfig, params atc.Params, version atc.Version) Step {
	return resourceStep{
		Tracker: factory.resourceTracker,
		Type:    resource.ResourceType(config.Type),

		Action: func(r resource.Resource, s ArtifactSource) resource.VersionedSource {
			return r.Get(resource.IOConfig(ioConfig), config.Source, params, version)
		},
	}
}

func (factory *gardenFactory) Put(ioConfig IOConfig, config atc.ResourceConfig, params atc.Params) Step {
	return resourceStep{
		Tracker: factory.resourceTracker,
		Type:    resource.ResourceType(config.Type),

		Action: func(r resource.Resource, s ArtifactSource) resource.VersionedSource {
			return r.Put(resource.IOConfig(ioConfig), config.Source, params, resourceSource{s})
		},
	}
}

func (factory *gardenFactory) Execute(ioConfig IOConfig, configSource BuildConfigSource) Step {
	return executeStep{
		IOConfig:     ioConfig,
		GardenClient: factory.gardenClient,
		ConfigSource: configSource,
	}
}
