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
	buildStepDelegate BuildStepDelegate,
) StepFactory {
	workerMetadata.WorkingDirectory = resource.ResourcesDir("get")

	variables := factory.variablesFactory.NewVariables(build.TeamName(), build.PipelineName())

	getAction := &GetAction{
		Type:          plan.Get.Type,
		Name:          plan.Get.Name,
		Resource:      plan.Get.Resource,
		Source:        creds.NewSource(variables, plan.Get.Source),
		Params:        creds.NewParams(variables, plan.Get.Params),
		VersionSource: NewVersionSourceFromPlan(plan.Get, factory.putActions),
		Tags:          plan.Get.Tags,
		Outputs:       []string{plan.Get.Name},

		buildStepDelegate:      buildStepDelegate,
		resourceFetcher:        factory.resourceFetcher,
		teamID:                 build.TeamID(),
		buildID:                build.ID(),
		planID:                 plan.ID,
		containerMetadata:      workerMetadata,
		dbResourceCacheFactory: factory.dbResourceCacheFactory,
		stepMetadata:           stepMetadata,

		// TODO: remove after all actions are introduced
		resourceTypes: creds.NewVersionedResourceTypes(variables, plan.Get.VersionedResourceTypes),
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
	buildStepDelegate BuildStepDelegate,
) StepFactory {
	workerMetadata.WorkingDirectory = resource.ResourcesDir("put")

	variables := factory.variablesFactory.NewVariables(build.TeamName(), build.PipelineName())

	putAction := &PutAction{
		Type:     plan.Put.Type,
		Name:     plan.Put.Name,
		Resource: plan.Put.Resource,
		Source:   creds.NewSource(variables, plan.Put.Source),
		Params:   creds.NewParams(variables, plan.Put.Params),
		Tags:     plan.Put.Tags,

		buildStepDelegate: buildStepDelegate,
		resourceFactory:   factory.resourceFactory,
		teamID:            build.TeamID(),
		buildID:           build.ID(),
		planID:            plan.ID,
		containerMetadata: workerMetadata,
		stepMetadata:      stepMetadata,

		resourceTypes: creds.NewVersionedResourceTypes(variables, plan.Put.VersionedResourceTypes),
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
	buildStepDelegate BuildStepDelegate,
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

	taskConfigFetcher = ValidatingConfigFetcher{ConfigFetcher: taskConfigFetcher}

	taskConfigFetcher = DeprecationConfigFetcher{
		Delegate: taskConfigFetcher,
		Stderr:   buildStepDelegate.Stderr(),
	}

	fetchConfigAction := &FetchConfigAction{
		configFetcher: taskConfigFetcher,
	}

	configSource := &FetchConfigActionTaskConfigSource{
		Action: fetchConfigAction,
	}

	variables := factory.variablesFactory.NewVariables(build.TeamName(), build.PipelineName())

	taskAction := &TaskAction{
		privileged:    Privileged(plan.Task.Privileged),
		configSource:  configSource,
		tags:          plan.Task.Tags,
		inputMapping:  plan.Task.InputMapping,
		outputMapping: plan.Task.OutputMapping,

		artifactsRoot:     workingDirectory,
		imageArtifactName: plan.Task.ImageArtifactName,

		buildEventsDelegate: taskBuildEventsDelegate,
		buildStepDelegate:   buildStepDelegate,
		workerPool:          factory.workerClient,
		teamID:              build.TeamID(),
		buildID:             build.ID(),
		jobID:               build.JobID(),
		stepName:            plan.Task.Name,
		planID:              plan.ID,
		containerMetadata:   containerMetadata,

		resourceTypes: creds.NewVersionedResourceTypes(variables, plan.Task.VersionedResourceTypes),

		variables: variables,
	}

	actions := []Action{fetchConfigAction, taskAction}

	return TaskStepFactory{NewActionsStep(logger, actions, buildEventsDelegate)}
}

func (factory *gardenFactory) taskWorkingDirectory(sourceName worker.ArtifactName) string {
	sum := sha1.Sum([]byte(sourceName))
	return filepath.Join("/tmp", "build", fmt.Sprintf("%x", sum[:4]))
}
