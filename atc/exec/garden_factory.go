package exec

import (
	"crypto/sha1"
	"fmt"
	"path/filepath"

	"code.cloudfoundry.org/lager"

	boshtemplate "github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/worker"
)

type gardenFactory struct {
	pool                  worker.Pool
	resourceFetcher       resource.Fetcher
	resourceCacheFactory  db.ResourceCacheFactory
	resourceConfigFactory db.ResourceConfigFactory
	variablesFactory      creds.VariablesFactory
	defaultLimits         atc.ContainerLimits
	strategy              worker.ContainerPlacementStrategy
	resourceFactory       resource.ResourceFactory
}

func NewGardenFactory(
	pool worker.Pool,
	resourceFetcher resource.Fetcher,
	resourceCacheFactory db.ResourceCacheFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	variablesFactory creds.VariablesFactory,
	defaultLimits atc.ContainerLimits,
	strategy worker.ContainerPlacementStrategy,
	resourceFactory resource.ResourceFactory,
) Factory {
	return &gardenFactory{
		pool:                  pool,
		resourceFetcher:       resourceFetcher,
		resourceCacheFactory:  resourceCacheFactory,
		resourceConfigFactory: resourceConfigFactory,
		variablesFactory:      variablesFactory,
		defaultLimits:         defaultLimits,
		strategy:              strategy,
		resourceFactory:       resourceFactory,
	}
}

func (factory *gardenFactory) Get(
	logger lager.Logger,
	plan atc.Plan,
	build db.Build,
	stepMetadata StepMetadata,
	workerMetadata db.ContainerMetadata,
	delegate GetDelegate,
) Step {
	workerMetadata.WorkingDirectory = resource.ResourcesDir("get")

	variables := factory.variablesFactory.NewVariables(build.TeamName(), build.PipelineName())

	getStep := NewGetStep(
		build,

		plan.Get.Name,
		plan.Get.Type,
		plan.Get.Resource,
		creds.NewSource(variables, plan.Get.Source),
		creds.NewParams(variables, plan.Get.Params),
		NewVersionSourceFromPlan(plan.Get),
		plan.Get.Tags,

		delegate,
		factory.resourceFetcher,
		build.TeamID(),
		build.ID(),
		plan.ID,
		workerMetadata,
		factory.resourceCacheFactory,
		stepMetadata,

		creds.NewVersionedResourceTypes(variables, plan.Get.VersionedResourceTypes),

		factory.strategy,
		factory.pool,
	)

	return LogError(getStep, delegate)
}

func (factory *gardenFactory) Put(
	logger lager.Logger,
	plan atc.Plan,
	build db.Build,
	stepMetadata StepMetadata,
	workerMetadata db.ContainerMetadata,
	delegate PutDelegate,
) Step {
	workerMetadata.WorkingDirectory = resource.ResourcesDir("put")

	variables := factory.variablesFactory.NewVariables(build.TeamName(), build.PipelineName())

	var putInputs PutInputs
	if plan.Put.Inputs == nil {
		// Put step defaults to all inputs if not specified
		putInputs = NewAllInputs()
	} else if plan.Put.Inputs.All {
		putInputs = NewAllInputs()
	} else {
		// Covers both cases where inputs are specified and when there are no
		// inputs specified and "all" field is given a false boolean, which will
		// result in no inputs attached
		putInputs = NewSpecificInputs(plan.Put.Inputs.Specified)
	}

	putStep := NewPutStep(
		build,

		plan.Put.Name,
		plan.Put.Type,
		plan.Put.Resource,
		creds.NewSource(variables, plan.Put.Source),
		creds.NewParams(variables, plan.Put.Params),
		plan.Put.Tags,
		putInputs,

		delegate,
		factory.pool,
		factory.resourceConfigFactory,
		plan.ID,
		workerMetadata,
		stepMetadata,

		creds.NewVersionedResourceTypes(variables, plan.Put.VersionedResourceTypes),

		factory.strategy,
		factory.resourceFactory,
	)

	return LogError(putStep, delegate)
}

func (factory *gardenFactory) Task(
	logger lager.Logger,
	plan atc.Plan,
	build db.Build,
	containerMetadata db.ContainerMetadata,
	delegate TaskDelegate,
) Step {
	workingDirectory := factory.taskWorkingDirectory(worker.ArtifactName(plan.Task.Name))
	containerMetadata.WorkingDirectory = workingDirectory

	credMgrVariables := factory.variablesFactory.NewVariables(build.TeamName(), build.PipelineName())

	var taskConfigSource TaskConfigSource
	var taskVars []boshtemplate.Variables
	if plan.Task.ConfigPath != "" {
		// external task - construct a source which reads it from file
		taskConfigSource = FileConfigSource{ConfigPath: plan.Task.ConfigPath}

		// for interpolation - use 'vars' from the pipeline, and then fill remaining with cred mgr variables
		taskVars = []boshtemplate.Variables{boshtemplate.StaticVariables(plan.Task.Vars), credMgrVariables}
	} else {
		// embedded task - first we take it
		taskConfigSource = StaticConfigSource{Config: plan.Task.Config}

		// for interpolation - use just cred mgr variables
		taskVars = []boshtemplate.Variables{credMgrVariables}
	}

	// override params
	taskConfigSource = &OverrideParamsConfigSource{ConfigSource: taskConfigSource, Params: plan.Task.Params}

	// interpolate template vars
	taskConfigSource = InterpolateTemplateConfigSource{ConfigSource: taskConfigSource, Vars: taskVars}

	// validate
	taskConfigSource = ValidatingConfigSource{ConfigSource: taskConfigSource}

	taskStep := NewTaskStep(
		Privileged(plan.Task.Privileged),
		taskConfigSource,
		plan.Task.Tags,
		plan.Task.InputMapping,
		plan.Task.OutputMapping,

		workingDirectory,
		plan.Task.ImageArtifactName,

		delegate,

		factory.pool,
		build.TeamID(),
		build.ID(),
		build.JobID(),
		plan.Task.Name,
		plan.ID,
		containerMetadata,

		creds.NewVersionedResourceTypes(credMgrVariables, plan.Task.VersionedResourceTypes),
		factory.defaultLimits,
		factory.strategy,
	)

	return LogError(taskStep, delegate)
}

func (factory *gardenFactory) taskWorkingDirectory(sourceName worker.ArtifactName) string {
	sum := sha1.Sum([]byte(sourceName))
	return filepath.Join("/tmp", "build", fmt.Sprintf("%x", sum[:4]))
}
