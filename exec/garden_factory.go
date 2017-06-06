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

func (factory *gardenFactory) Get(
	logger lager.Logger,
	buildID int,
	teamID int,
	plan atc.Plan,
	stepMetadata StepMetadata,
	workerMetadata db.ContainerMetadata,
	buildDelegate BuildDelegate,
) StepFactory {
	workerMetadata.WorkingDirectory = resource.ResourcesDir("get")

	imageFetchingDelegate := buildDelegate.ImageFetchingDelegate(plan.ID)
	getAction := &GetAction{
		Type:     plan.Get.Type,
		Name:     plan.Get.Name,
		Resource: plan.Get.Resource,
		Source:   plan.Get.Source,
		Params:   plan.Get.Params,
		Version:  plan.Get.Version,
		Tags:     plan.Get.Tags,
		Outputs:  []string{plan.Get.Name},

		// TODO: can we remove these dependencies?
		imageFetchingDelegate: imageFetchingDelegate,
		resourceFetcher:       factory.resourceFetcher,
		teamID:                teamID,
		containerMetadata:     workerMetadata,
		resourceInstance: resource.NewResourceInstance(
			resource.ResourceType(plan.Get.Type),
			plan.Get.Version,
			plan.Get.Source,
			plan.Get.Params,
			db.ForBuild(buildID),
			db.NewBuildStepContainerOwner(buildID, plan.ID),
			plan.Get.VersionedResourceTypes,
			factory.dbResourceCacheFactory,
		),
		stepMetadata: stepMetadata,

		// TODO: remove after all actions are introduced
		resourceTypes: plan.Get.VersionedResourceTypes,
	}

	actions := []Action{getAction}

	buildEventsDelegate := buildDelegate.GetBuildEventsDelegate(plan.ID, *plan.Get, getAction)
	return newActionsStep(logger, actions, buildEventsDelegate)
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
	result *atc.Version,
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
		result,
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
