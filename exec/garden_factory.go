package exec

import (
	"path/filepath"

	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

//go:generate counterfeiter . TrackerFactory

type TrackerFactory interface {
	TrackerFor(worker.Client) resource.Tracker
}

type gardenFactory struct {
	workerClient   worker.Client
	trackerFactory TrackerFactory
	uuidGenerator  UUIDGenFunc
}

type UUIDGenFunc func() string

func NewGardenFactory(
	workerClient worker.Client,
	trackerFactory TrackerFactory,
	uuidGenerator UUIDGenFunc,
) Factory {
	return &gardenFactory{
		workerClient:   workerClient,
		trackerFactory: trackerFactory,
		uuidGenerator:  uuidGenerator,
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
		factory.workerClient,
		resourceConfig,
		params,
		stepMetadata,
		resource.Session{
			ID:        id,
			Ephemeral: false,
		},
		tags,
		delegate,
		factory.trackerFactory,
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
	return newGetStep(
		logger,
		sourceName,
		factory.workerClient,
		resourceConfig,
		version,
		params,
		stepMetadata,
		resource.Session{
			ID:        id,
			Ephemeral: false,
		},
		tags,
		delegate,
		factory.trackerFactory,
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
	return newPutStep(
		logger,
		factory.workerClient,
		resourceConfig,
		params,
		stepMetadata,
		resource.Session{
			ID:        id,
			Ephemeral: false,
		},
		tags,
		delegate,
		factory.trackerFactory,
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
	return newTaskStep(
		sourceName,
		id,
		tags,
		delegate,
		privileged,
		configSource,
		factory.workerClient,
		filepath.Join("/tmp", "build", factory.uuidGenerator()),
	)
}
