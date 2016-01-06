package exec

import (
	"crypto/sha1"
	"fmt"
	"path/filepath"

	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

type gardenFactory struct {
	workerClient worker.Client
	tracker      resource.Tracker
}

type UUIDGenFunc func() string

func NewGardenFactory(
	workerClient worker.Client,
	tracker resource.Tracker,
) Factory {
	return &gardenFactory{
		workerClient: workerClient,
		tracker:      tracker,
	}
}

func (factory *gardenFactory) DependentGet(
	logger lager.Logger,
	stepMetadata StepMetadata,
	sourceName SourceName,
	id worker.Identifier,
	delegate GetDelegate,
	resourceConfig atc.ResourceConfig,
	tags atc.Tags,
	params atc.Params,
) StepFactory {
	return newDependentGetStep(
		logger,
		sourceName,
		resourceConfig,
		params,
		stepMetadata,
		resource.Session{
			ID:        id,
			Ephemeral: false,
		},
		tags,
		delegate,
		factory.tracker,
	)
}

func (factory *gardenFactory) Get(
	logger lager.Logger,
	stepMetadata StepMetadata,
	sourceName SourceName,
	id worker.Identifier,
	delegate GetDelegate,
	resourceConfig atc.ResourceConfig,
	params atc.Params,
	tags atc.Tags,
	version atc.Version,
) StepFactory {
	id.WorkingDirectory = resource.ResourcesDir("get")
	return newGetStep(
		logger,
		sourceName,
		resourceConfig,
		version,
		params,
		resource.ResourceCacheIdentifier{
			Type:    resource.ResourceType(resourceConfig.Type),
			Source:  resourceConfig.Source,
			Params:  params,
			Version: version,
		},
		stepMetadata,
		resource.Session{
			ID:        id,
			Ephemeral: false,
		},
		tags,
		delegate,
		factory.tracker,
	)
}

func (factory *gardenFactory) Put(
	logger lager.Logger,
	stepMetadata StepMetadata,
	id worker.Identifier,
	delegate PutDelegate,
	resourceConfig atc.ResourceConfig,
	tags atc.Tags,
	params atc.Params,
) StepFactory {
	id.WorkingDirectory = resource.ResourcesDir("put")
	return newPutStep(
		logger,
		resourceConfig,
		params,
		stepMetadata,
		resource.Session{
			ID:        id,
			Ephemeral: false,
		},
		tags,
		delegate,
		factory.tracker,
	)
}

func (factory *gardenFactory) Task(
	logger lager.Logger,
	sourceName SourceName,
	id worker.Identifier,
	delegate TaskDelegate,
	privileged Privileged,
	tags atc.Tags,
	configSource TaskConfigSource,
) StepFactory {
	id.WorkingDirectory = factory.taskWorkingDirectory(sourceName)
	return newTaskStep(
		logger,
		sourceName,
		id,
		tags,
		delegate,
		privileged,
		configSource,
		factory.workerClient,
		id.WorkingDirectory,
	)
}

func (factory *gardenFactory) taskWorkingDirectory(sourceName SourceName) string {
	sum := sha1.Sum([]byte(sourceName))
	return filepath.Join("/tmp", "build", fmt.Sprintf("%x", sum[:4]))
}
