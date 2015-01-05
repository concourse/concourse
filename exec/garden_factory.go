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

func (factory *gardenFactory) Get(sessionID SessionID, ioConfig IOConfig, config atc.ResourceConfig, params atc.Params, version atc.Version) Step {
	return resourceStep{
		SessionID: resource.SessionID(sessionID),

		Tracker: factory.resourceTracker,
		Type:    resource.ResourceType(config.Type),

		Action: func(r resource.Resource, s ArtifactSource) resource.VersionedSource {
			return r.Get(resource.IOConfig(ioConfig), config.Source, params, version)
		},
	}
}

func (factory *gardenFactory) Put(sessionID SessionID, ioConfig IOConfig, config atc.ResourceConfig, params atc.Params) Step {
	return resourceStep{
		SessionID: resource.SessionID(sessionID),

		Tracker: factory.resourceTracker,
		Type:    resource.ResourceType(config.Type),

		Action: func(r resource.Resource, s ArtifactSource) resource.VersionedSource {
			return r.Put(resource.IOConfig(ioConfig), config.Source, params, resourceSource{s})
		},
	}
}

func (factory *gardenFactory) Execute(sessionID SessionID, ioConfig IOConfig, configSource BuildConfigSource) Step {
	return executeStep{
		SessionID: sessionID,

		IOConfig:     ioConfig,
		ConfigSource: configSource,

		GardenClient: factory.gardenClient,
	}
}
