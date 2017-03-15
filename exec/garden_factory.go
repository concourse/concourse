package exec

import (
	"crypto/sha1"
	"fmt"
	"path/filepath"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

type gardenFactory struct {
	workerClient           worker.Client
	resourceFetcher        resource.Fetcher
	resourceFactory        resource.ResourceFactory
	dbResourceCacheFactory dbng.ResourceCacheFactory
}

func NewGardenFactory(
	workerClient worker.Client,
	resourceFetcher resource.Fetcher,
	resourceFactory resource.ResourceFactory,
	dbResourceCacheFactory dbng.ResourceCacheFactory,
) Factory {
	return &gardenFactory{
		workerClient:           workerClient,
		resourceFetcher:        resourceFetcher,
		resourceFactory:        resourceFactory,
		dbResourceCacheFactory: dbResourceCacheFactory,
	}
}

func (factory *gardenFactory) DependentGet(
	logger lager.Logger,
	stepMetadata StepMetadata,
	sourceName worker.ArtifactName,
	id worker.Identifier,
	workerMetadata worker.Metadata,
	delegate GetDelegate,
	resourceConfig atc.ResourceConfig,
	tags atc.Tags,
	teamID int,
	params atc.Params,
	resourceTypes atc.VersionedResourceTypes,
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
		teamID,
		delegate,
		factory.resourceFetcher,
		resourceTypes,
		factory.dbResourceCacheFactory,
	)
}

func (factory *gardenFactory) Get(
	logger lager.Logger,
	stepMetadata StepMetadata,
	sourceName worker.ArtifactName,
	id worker.Identifier,
	workerMetadata worker.Metadata,
	delegate GetDelegate,
	resourceConfig atc.ResourceConfig,
	tags atc.Tags,
	teamID int,
	params atc.Params,
	version atc.Version,
	resourceTypes atc.VersionedResourceTypes,
) StepFactory {
	workerMetadata.WorkingDirectory = resource.ResourcesDir("get")
	return newGetStep(
		logger,
		sourceName,
		resourceConfig,
		version,
		params,
		resource.NewResourceInstance(
			resource.ResourceType(resourceConfig.Type),
			version,
			resourceConfig.Source,
			params,
			dbng.ForBuild{BuildID: id.BuildID},
			resourceTypes,
			factory.dbResourceCacheFactory,
		),
		stepMetadata,
		resource.Session{
			ID:        id,
			Metadata:  workerMetadata,
			Ephemeral: false,
		},
		tags,
		teamID,
		delegate,
		factory.resourceFetcher,
		resourceTypes,
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
	teamID int,
	params atc.Params,
	resourceTypes atc.VersionedResourceTypes,
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
		teamID,
		delegate,
		factory.resourceFactory,
		resourceTypes,
	)
}

func (factory *gardenFactory) Task(
	logger lager.Logger,
	sourceName worker.ArtifactName,
	id worker.Identifier,
	workerMetadata worker.Metadata,
	delegate TaskDelegate,
	privileged Privileged,
	tags atc.Tags,
	teamID int,
	configSource TaskConfigSource,
	resourceTypes atc.VersionedResourceTypes,
	inputMapping map[string]string,
	outputMapping map[string]string,
	imageArtifactName string,
	clock clock.Clock,
) StepFactory {
	workingDirectory := factory.taskWorkingDirectory(sourceName)
	workerMetadata.WorkingDirectory = workingDirectory
	return newTaskStep(
		logger,
		id,
		workerMetadata,
		tags,
		teamID,
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
	)
}

func (factory *gardenFactory) taskWorkingDirectory(sourceName worker.ArtifactName) string {
	sum := sha1.Sum([]byte(sourceName))
	return filepath.Join("/tmp", "build", fmt.Sprintf("%x", sum[:4]))
}
