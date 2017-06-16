package exec

import (
	"crypto/sha1"
	"fmt"
	"path/filepath"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

type gardenFactory struct {
	workerClient           worker.Client
	resourceFetcher        resource.Fetcher
	resourceFactory        resource.ResourceFactory
	dbResourceCacheFactory db.ResourceCacheFactory
	variablesFactory       creds.VariablesFactory

	putActions map[atc.PlanID]*PutAction
}

func NewGardenFactory(
	workerClient worker.Client,
	resourceFetcher resource.Fetcher,
	resourceFactory resource.ResourceFactory,
	dbResourceCacheFactory db.ResourceCacheFactory,
	variablesFactory creds.VariablesFactory,
) Factory {
	return &gardenFactory{
		workerClient:           workerClient,
		resourceFetcher:        resourceFetcher,
		resourceFactory:        resourceFactory,
		dbResourceCacheFactory: dbResourceCacheFactory,
		variablesFactory:       variablesFactory,
		putActions:             map[atc.PlanID]*PutAction{},
	}
}

type GetStepFactory struct {
	ActionsStep
}
type GetStep Step

func (s GetStepFactory) Using(repository *worker.ArtifactRepository) Step {
	return GetStep(s.ActionsStep.Using(repository))
}

type PutStepFactory struct {
	ActionsStep
}

type PutStep Step

func (s PutStepFactory) Using(repository *worker.ArtifactRepository) Step {
	return PutStep(s.ActionsStep.Using(repository))
}

type TaskStepFactory struct {
	ActionsStep
}

type TaskStep Step

func (s TaskStepFactory) Using(repository *worker.ArtifactRepository) Step {
	return TaskStep(s.ActionsStep.Using(repository))
}

func (factory *gardenFactory) Get(
	logger lager.Logger,
	plan atc.Plan,
	build db.Build,
	stepMetadata StepMetadata,
	workerMetadata db.ContainerMetadata,
	buildEventsDelegate ActionsBuildEventsDelegate,
	imageFetchingDelegate ImageFetchingDelegate,
) StepFactory {
	workerMetadata.WorkingDirectory = resource.ResourcesDir("get")

	source := creds.NewSource(factory.variablesFactory.NewVariables(build.TeamName(), build.PipelineName()), plan.Get.Source)

	getAction := &GetAction{
		Type:          plan.Get.Type,
		Name:          plan.Get.Name,
		Resource:      plan.Get.Resource,
		Source:        source,
		Params:        plan.Get.Params,
		VersionSource: NewVersionSourceFromPlan(plan.Get, factory.putActions),
		Tags:          plan.Get.Tags,
		Outputs:       []string{plan.Get.Name},

		imageFetchingDelegate:  imageFetchingDelegate,
		resourceFetcher:        factory.resourceFetcher,
		teamID:                 build.TeamID(),
		buildID:                build.ID(),
		planID:                 plan.ID,
		containerMetadata:      workerMetadata,
		dbResourceCacheFactory: factory.dbResourceCacheFactory,
		stepMetadata:           stepMetadata,

		// TODO: remove after all actions are introduced
		resourceTypes: plan.Get.VersionedResourceTypes,
	}

	actions := []Action{getAction}

	return GetStepFactory{NewActionsStep(logger, actions, buildEventsDelegate)}
}

func (factory *gardenFactory) Put(
	logger lager.Logger,
	plan atc.Plan,
	build db.Build,
	stepMetadata StepMetadata,
	workerMetadata db.ContainerMetadata,
	buildEventsDelegate ActionsBuildEventsDelegate,
	imageFetchingDelegate ImageFetchingDelegate,
) StepFactory {
	workerMetadata.WorkingDirectory = resource.ResourcesDir("put")

	source := creds.NewSource(factory.variablesFactory.NewVariables(build.TeamName(), build.PipelineName()), plan.Put.Source)

	putAction := &PutAction{
		Type:     plan.Put.Type,
		Name:     plan.Put.Name,
		Resource: plan.Put.Resource,
		Source:   source,
		Params:   plan.Put.Params,
		Tags:     plan.Put.Tags,

		imageFetchingDelegate: imageFetchingDelegate,
		resourceFactory:       factory.resourceFactory,
		teamID:                build.TeamID(),
		buildID:               build.ID(),
		planID:                plan.ID,
		containerMetadata:     workerMetadata,
		stepMetadata:          stepMetadata,

		resourceTypes: plan.Put.VersionedResourceTypes,
	}
	factory.putActions[plan.ID] = putAction

	actions := []Action{putAction}

	return PutStepFactory{NewActionsStep(logger, actions, buildEventsDelegate)}
}

func (factory *gardenFactory) Task(
	logger lager.Logger,
	plan atc.Plan,
	build db.Build,
	containerMetadata db.ContainerMetadata,
	taskBuildEventsDelegate TaskBuildEventsDelegate,
	buildEventsDelegate ActionsBuildEventsDelegate,
	imageFetchingDelegate ImageFetchingDelegate,
) StepFactory {
	workingDirectory := factory.taskWorkingDirectory(worker.ArtifactName(plan.Task.Name))
	containerMetadata.WorkingDirectory = workingDirectory

	var taskConfigFetcher TaskConfigFetcher
	if plan.Task.ConfigPath != "" && (plan.Task.Config != nil || plan.Task.Params != nil) {
		taskConfigFetcher = MergedConfigFetcher{
			A: FileConfigFetcher{plan.Task.ConfigPath},
			B: StaticConfigFetcher{Plan: *plan.Task},
		}
	} else if plan.Task.Config != nil {
		taskConfigFetcher = StaticConfigFetcher{Plan: *plan.Task}
	} else if plan.Task.ConfigPath != "" {
		taskConfigFetcher = FileConfigFetcher{plan.Task.ConfigPath}
	}

	// XXX: Maybe we can add comments to why we are using the same variable or use new ones?
	// Wrap the task config fetcher with a validator
	taskConfigFetcher = ValidatingConfigFetcher{ConfigFetcher: taskConfigFetcher}
	// XXX: Whats the purpose of this? Is this serving some deprecated or too-be-deprecated
	// functionality?
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

		buildEventsDelegate:   taskBuildEventsDelegate,
		imageFetchingDelegate: imageFetchingDelegate,
		workerPool:            factory.workerClient,
		teamID:                build.TeamID(),
		buildID:               build.ID(),
		planID:                plan.ID,
		containerMetadata:     containerMetadata,

		resourceTypes: plan.Task.VersionedResourceTypes,
	}

	actions := []Action{fetchConfigAction, taskAction}

	return TaskStepFactory{NewActionsStep(logger, actions, buildEventsDelegate)}
}

func (factory *gardenFactory) taskWorkingDirectory(sourceName worker.ArtifactName) string {
	sum := sha1.Sum([]byte(sourceName))
	return filepath.Join("/tmp", "build", fmt.Sprintf("%x", sum[:4]))
}
