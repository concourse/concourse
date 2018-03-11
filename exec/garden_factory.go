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

func (factory *gardenFactory) Get(
	logger lager.Logger,
	plan atc.Plan,
	build db.Build,
	stepMetadata StepMetadata,
	workerMetadata db.ContainerMetadata,
	buildEventsDelegate ActionsBuildEventsDelegate,
	buildStepDelegate BuildStepDelegate,
) Step {
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

	return NewActionsStep(logger, actions, buildEventsDelegate)
}

func (factory *gardenFactory) Put(
	logger lager.Logger,
	plan atc.Plan,
	build db.Build,
	stepMetadata StepMetadata,
	workerMetadata db.ContainerMetadata,
	buildEventsDelegate ActionsBuildEventsDelegate,
	buildStepDelegate BuildStepDelegate,
) Step {
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

	return NewActionsStep(logger, actions, buildEventsDelegate)
}

func (factory *gardenFactory) Task(
	logger lager.Logger,
	plan atc.Plan,
	build db.Build,
	containerMetadata db.ContainerMetadata,
	taskDelegate TaskDelegate,
) Step {
	workingDirectory := factory.taskWorkingDirectory(worker.ArtifactName(plan.Task.Name))
	containerMetadata.WorkingDirectory = workingDirectory

	var taskConfigSource TaskConfigSource
	if plan.Task.ConfigPath != "" && (plan.Task.Config != nil || plan.Task.Params != nil) {
		taskConfigSource = MergedConfigSource{
			A: FileConfigSource{plan.Task.ConfigPath},
			B: StaticConfigSource{Plan: *plan.Task},
		}
	} else if plan.Task.Config != nil {
		taskConfigSource = StaticConfigSource{Plan: *plan.Task}
	} else if plan.Task.ConfigPath != "" {
		taskConfigSource = FileConfigSource{plan.Task.ConfigPath}
	}

	taskConfigSource = ValidatingConfigSource{ConfigSource: taskConfigSource}

	taskConfigSource = DeprecationConfigSource{
		Delegate: taskConfigSource,
		Stderr:   taskDelegate.Stderr(),
	}

	variables := factory.variablesFactory.NewVariables(build.TeamName(), build.PipelineName())

	taskStep := NewTaskStep(
		Privileged(plan.Task.Privileged),
		taskConfigSource,
		plan.Task.Tags,
		plan.Task.InputMapping,
		plan.Task.OutputMapping,

		workingDirectory,
		plan.Task.ImageArtifactName,

		taskDelegate,

		factory.workerClient,
		build.TeamID(),
		build.ID(),
		build.JobID(),
		plan.Task.Name,
		plan.ID,
		containerMetadata,

		creds.NewVersionedResourceTypes(variables, plan.Task.VersionedResourceTypes),
		variables,
	)

	return LogError(taskStep, taskDelegate)
}

func (factory *gardenFactory) taskWorkingDirectory(sourceName worker.ArtifactName) string {
	sum := sha1.Sum([]byte(sourceName))
	return filepath.Join("/tmp", "build", fmt.Sprintf("%x", sum[:4]))
}
