package exec

import (
	"crypto/sha1"
	"fmt"
	"path/filepath"

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

	putActions map[atc.PlanID]*PutAction
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
		putActions:             map[atc.PlanID]*PutAction{},
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
		Type:          plan.Get.Type,
		Name:          plan.Get.Name,
		Resource:      plan.Get.Resource,
		Source:        plan.Get.Source,
		Params:        plan.Get.Params,
		VersionSource: NewVersionSourceFromPlan(plan.Get, factory.putActions),
		Tags:          plan.Get.Tags,
		Outputs:       []string{plan.Get.Name},

		imageFetchingDelegate:  imageFetchingDelegate,
		resourceFetcher:        factory.resourceFetcher,
		teamID:                 teamID,
		buildID:                buildID,
		planID:                 plan.ID,
		containerMetadata:      workerMetadata,
		dbResourceCacheFactory: factory.dbResourceCacheFactory,
		stepMetadata:           stepMetadata,

		// TODO: remove after all actions are introduced
		resourceTypes: plan.Get.VersionedResourceTypes,
	}

	actions := []Action{getAction}

	buildEventsDelegate := buildDelegate.GetBuildEventsDelegate(plan.ID, *plan.Get)
	return newActionsStep(logger, actions, buildEventsDelegate)
}

func (factory *gardenFactory) Put(
	logger lager.Logger,
	teamID int,
	buildID int,
	plan atc.Plan,
	stepMetadata StepMetadata,
	workerMetadata db.ContainerMetadata,
	buildDelegate BuildDelegate,
) StepFactory {
	workerMetadata.WorkingDirectory = resource.ResourcesDir("put")

	imageFetchingDelegate := buildDelegate.ImageFetchingDelegate(plan.ID)
	putAction := &PutAction{
		Type:     plan.Put.Type,
		Name:     plan.Put.Name,
		Resource: plan.Put.Resource,
		Source:   plan.Put.Source,
		Params:   plan.Put.Params,
		Tags:     plan.Put.Tags,

		imageFetchingDelegate: imageFetchingDelegate,
		resourceFactory:       factory.resourceFactory,
		teamID:                teamID,
		buildID:               buildID,
		planID:                plan.ID,
		containerMetadata:     workerMetadata,
		stepMetadata:          stepMetadata,

		resourceTypes: plan.Put.VersionedResourceTypes,
	}
	factory.putActions[plan.ID] = putAction

	actions := []Action{putAction}

	buildEventsDelegate := buildDelegate.PutBuildEventsDelegate(plan.ID, *plan.Put)
	return newActionsStep(logger, actions, buildEventsDelegate)
}

func (factory *gardenFactory) Task(
	logger lager.Logger,
	plan atc.Plan,
	teamID int,
	buildID int,
	containerMetadata db.ContainerMetadata,
	buildDelegate BuildDelegate,
) StepFactory {
	workingDirectory := factory.taskWorkingDirectory(worker.ArtifactName(plan.Task.Name))
	containerMetadata.WorkingDirectory = workingDirectory

	imageFetchingDelegate := buildDelegate.ImageFetchingDelegate(plan.ID)

	var taskConfigFetcher TaskConfigFetcher
	if plan.Task.ConfigPath != "" && (plan.Task.Config != nil || plan.Task.Params != nil) {
		logger.Debug("config fetcher merge")
		taskConfigFetcher = MergedConfigFetcher{
			A: FileConfigFetcher{plan.Task.ConfigPath},
			B: StaticConfigFetcher{Plan: *plan.Task},
		}
	} else if plan.Task.Config != nil {
		logger.Debug("config fetcher static")
		taskConfigFetcher = StaticConfigFetcher{Plan: *plan.Task}
	} else if plan.Task.ConfigPath != "" {
		logger.Debug("config fetcher config path")
		taskConfigFetcher = FileConfigFetcher{plan.Task.ConfigPath}
	}

	taskConfigFetcher = ValidatingConfigFetcher{ConfigFetcher: taskConfigFetcher}
	taskConfigFetcher = DeprecationConfigFetcher{
		Delegate: taskConfigFetcher,
		Stderr:   imageFetchingDelegate.Stderr(),
	}

	fetchConfigAction := &FetchConfigAction{
		configFetcher: taskConfigFetcher,
	}

	configSource := &FetchConfigActionTaskConfigSource{
		Action: fetchConfigAction,
	}

	taskAction := &TaskAction{
		privileged:    Privileged(plan.Task.Privileged),
		configSource:  configSource,
		tags:          plan.Task.Tags,
		inputMapping:  plan.Task.InputMapping,
		outputMapping: plan.Task.OutputMapping,

		artifactsRoot:     workingDirectory,
		imageArtifactName: plan.Task.ImageArtifactName,

		imageFetchingDelegate: imageFetchingDelegate,
		workerPool:            factory.workerClient,
		teamID:                teamID,
		buildID:               buildID,
		planID:                plan.ID,
		containerMetadata:     containerMetadata,

		resourceTypes: plan.Task.VersionedResourceTypes,
	}

	actions := []Action{fetchConfigAction, taskAction}

	buildEventsDelegate := buildDelegate.TaskBuildEventsDelegate(plan.ID, *plan.Task)
	return newActionsStep(logger, actions, buildEventsDelegate)
}

func (factory *gardenFactory) taskWorkingDirectory(sourceName worker.ArtifactName) string {
	sum := sha1.Sum([]byte(sourceName))
	return filepath.Join("/tmp", "build", fmt.Sprintf("%x", sum[:4]))
}
