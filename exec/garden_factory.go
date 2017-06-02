package exec

import (
	"crypto/sha1"
	"fmt"
	"path/filepath"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

type gardenFactory struct {
	workerClient           worker.Client
	resourceFetcher        resource.Fetcher
	resourceFactory        resource.ResourceFactory
	dbResourceCacheFactory db.ResourceCacheFactory
}

func NewGardenFactory(
	workerClient worker.Client,
	resourceFetcher resource.Fetcher,
	resourceFactory resource.ResourceFactory,
	dbResourceCacheFactory db.ResourceCacheFactory,
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
	teamID int,
	buildID int,
	planID atc.PlanID,
	stepMetadata StepMetadata,
	sourceName worker.ArtifactName,
	workerMetadata db.ContainerMetadata,
	delegate GetDelegate,
	resourceConfig atc.ResourceConfig,
	tags atc.Tags,
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
			Metadata: workerMetadata,
		},
		tags,
		teamID,
		buildID,
		delegate,
		factory.resourceFetcher,
		resourceTypes,
		factory.dbResourceCacheFactory,
	)
}

func (factory *gardenFactory) Get(
	logger lager.Logger,
	teamID int,
	buildID int,
	planID atc.PlanID,
	stepMetadata StepMetadata,
	sourceName worker.ArtifactName,
	workerMetadata db.ContainerMetadata,
	delegate GetDelegate,
	resourceConfig atc.ResourceConfig,
	tags atc.Tags,
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
			db.ForBuild(buildID),
			resourceTypes,
			factory.dbResourceCacheFactory,
		),
		stepMetadata,
		resource.Session{
			Metadata: workerMetadata,
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
	teamID int,
	buildID int,
	planID atc.PlanID,
	stepMetadata StepMetadata,
	workerMetadata db.ContainerMetadata,
	delegate PutDelegate,
	resourceConfig atc.ResourceConfig,
	tags atc.Tags,
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
			Metadata: workerMetadata,
		},
		tags,
		teamID,
		buildID,
		planID,
		delegate,
		factory.resourceFactory,
		resourceTypes,
	)
}

func (factory *gardenFactory) Task(
	logger lager.Logger,
	teamID int,
	buildID int,
	planID atc.PlanID,
	sourceName worker.ArtifactName,
	workerMetadata db.ContainerMetadata,
	delegate TaskDelegate,
	privileged Privileged,
	tags atc.Tags,
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
		workerMetadata,
		tags,
		teamID,
		buildID,
		planID,
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
