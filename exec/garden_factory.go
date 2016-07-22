package exec

import (
	"crypto/sha1"
	"fmt"
	"path/filepath"
	"time"

	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

type gardenFactory struct {
	workerClient    worker.Client
	tracker         resource.Tracker
	resourceFetcher resource.Fetcher
}

//go:generate counterfeiter . TrackerFactory

type TrackerFactory interface {
	TrackerFor(client worker.Client) resource.Tracker
}

func NewGardenFactory(
	workerClient worker.Client,
	tracker resource.Tracker,
	resourceFetcher resource.Fetcher,
) Factory {
	return &gardenFactory{
		workerClient:    workerClient,
		tracker:         tracker,
		resourceFetcher: resourceFetcher,
	}
}

func (factory *gardenFactory) DependentGet(
	logger lager.Logger,
	stepMetadata StepMetadata,
	sourceName SourceName,
	id worker.Identifier,
	workerMetadata worker.Metadata,
	delegate GetDelegate,
	resourceConfig atc.ResourceConfig,
	tags atc.Tags,
	teamName string,
	params atc.Params,
	resourceTypes atc.ResourceTypes,
	containerSuccessTTL time.Duration,
	containerFailureTTL time.Duration,
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
			Metadata:  workerMetadata,
		},
		tags,
		teamName,
		delegate,
		factory.resourceFetcher,
		resourceTypes,
		containerSuccessTTL,
		containerFailureTTL,
	)
}

func (factory *gardenFactory) Get(
	logger lager.Logger,
	stepMetadata StepMetadata,
	sourceName SourceName,
	id worker.Identifier,
	workerMetadata worker.Metadata,
	delegate GetDelegate,
	resourceConfig atc.ResourceConfig,
	tags atc.Tags,
	teamName string,
	params atc.Params,
	version atc.Version,
	resourceTypes atc.ResourceTypes,
	containerSuccessTTL time.Duration,
	containerFailureTTL time.Duration,
) StepFactory {
	workerMetadata.WorkingDirectory = resource.ResourcesDir("get")
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
			Metadata:  workerMetadata,
			Ephemeral: false,
		},
		tags,
		teamName,
		delegate,
		factory.resourceFetcher,
		resourceTypes,

		containerSuccessTTL,
		containerFailureTTL,
	)
}

func (factory *gardenFactory) Put(
	logger lager.Logger,
	stepMetadata StepMetadata,
	id worker.Identifier,
	workerMetadata worker.Metadata,
	delegate PutDelegate,
	resourceConfig atc.ResourceConfig,
	tags atc.Tags,
	teamName string,
	params atc.Params,
	resourceTypes atc.ResourceTypes,
	containerSuccessTTL time.Duration,
	containerFailureTTL time.Duration,
) StepFactory {
	workerMetadata.WorkingDirectory = resource.ResourcesDir("put")
	return newPutStep(
		logger,
		resourceConfig,
		params,
		stepMetadata,
		resource.Session{
			ID:        id,
			Ephemeral: false,
			Metadata:  workerMetadata,
		},
		tags,
		teamName,
		delegate,
		factory.tracker,
		resourceTypes,
		containerSuccessTTL,
		containerFailureTTL,
	)
}

func (factory *gardenFactory) Task(
	logger lager.Logger,
	sourceName SourceName,
	id worker.Identifier,
	workerMetadata worker.Metadata,
	delegate TaskDelegate,
	privileged Privileged,
	tags atc.Tags,
	teamName string,
	configSource TaskConfigSource,
	resourceTypes atc.ResourceTypes,
	inputMapping map[string]string,
	outputMapping map[string]string,
	imageArtifactName string,
	clock clock.Clock,
	containerSuccessTTL time.Duration,
	containerFailureTTL time.Duration,
) StepFactory {
	workingDirectory := factory.taskWorkingDirectory(sourceName)
	workerMetadata.WorkingDirectory = workingDirectory
	return newTaskStep(
		logger,
		id,
		workerMetadata,
		tags,
		teamName,
		delegate,
		privileged,
		configSource,
		factory.workerClient,
		workingDirectory,
		resourceTypes,
		inputMapping,
		outputMapping,
		imageArtifactName,
		clock,
		containerSuccessTTL,
		containerFailureTTL,
	)
}

func (factory *gardenFactory) taskWorkingDirectory(sourceName SourceName) string {
	sum := sha1.Sum([]byte(sourceName))
	return filepath.Join("/tmp", "build", fmt.Sprintf("%x", sum[:4]))
}
