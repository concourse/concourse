package builder

import (
	"crypto/sha1"
	"fmt"
	"path/filepath"

	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/worker"
)

type stepFactory struct {
	pool                  worker.Pool
	client                worker.Client
	resourceFetcher       resource.Fetcher
	resourceCacheFactory  db.ResourceCacheFactory
	resourceConfigFactory db.ResourceConfigFactory
	variablesFactory      creds.VariablesFactory
	defaultLimits         atc.ContainerLimits
	strategy              worker.ContainerPlacementStrategy
	resourceFactory       resource.ResourceFactory
}

func NewStepFactory(
	pool worker.Pool,
	client worker.Client,
	resourceFetcher resource.Fetcher,
	resourceCacheFactory db.ResourceCacheFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	variablesFactory creds.VariablesFactory,
	defaultLimits atc.ContainerLimits,
	strategy worker.ContainerPlacementStrategy,
	resourceFactory resource.ResourceFactory,
) *stepFactory {
	return &stepFactory{
		pool:                  pool,
		client:                client,
		resourceFetcher:       resourceFetcher,
		resourceCacheFactory:  resourceCacheFactory,
		resourceConfigFactory: resourceConfigFactory,
		variablesFactory:      variablesFactory,
		defaultLimits:         defaultLimits,
		strategy:              strategy,
		resourceFactory:       resourceFactory,
	}
}

func (factory *stepFactory) GetStep(
	plan atc.Plan,
	build db.Build,
	stepMetadata exec.StepMetadata,
	workerMetadata db.ContainerMetadata,
	delegate exec.GetDelegate,
) exec.Step {
	workerMetadata.WorkingDirectory = resource.ResourcesDir("get")

	variables := factory.variablesFactory.NewVariables(build.TeamName(), build.PipelineName())

	getStep := exec.NewGetStep(
		build,

		plan.Get.Name,
		plan.Get.Type,
		plan.Get.Resource,
		creds.NewSource(variables, plan.Get.Source),
		creds.NewParams(variables, plan.Get.Params),
		exec.NewVersionSourceFromPlan(plan.Get),
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

	return exec.LogError(getStep, delegate)
}

func (factory *stepFactory) PutStep(
	plan atc.Plan,
	build db.Build,
	stepMetadata exec.StepMetadata,
	workerMetadata db.ContainerMetadata,
	delegate exec.PutDelegate,
) exec.Step {
	workerMetadata.WorkingDirectory = resource.ResourcesDir("put")

	variables := factory.variablesFactory.NewVariables(build.TeamName(), build.PipelineName())

	var putInputs exec.PutInputs
	if plan.Put.Inputs == nil {
		// Put step defaults to all inputs if not specified
		putInputs = exec.NewAllInputs()
	} else if plan.Put.Inputs.All {
		putInputs = exec.NewAllInputs()
	} else {
		// Covers both cases where inputs are specified and when there are no
		// inputs specified and "all" field is given a false boolean, which will
		// result in no inputs attached
		putInputs = exec.NewSpecificInputs(plan.Put.Inputs.Specified)
	}

	putStep := exec.NewPutStep(
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

	return exec.LogError(putStep, delegate)
}

func (factory *stepFactory) TaskStep(
	plan atc.Plan,
	build db.Build,
	containerMetadata db.ContainerMetadata,
	delegate exec.TaskDelegate,
) exec.Step {
	sum := sha1.Sum([]byte(plan.Task.Name))
	workingDirectory := filepath.Join("/tmp", "build", fmt.Sprintf("%x", sum[:4]))

	containerMetadata.WorkingDirectory = workingDirectory

	credMgrVariables := factory.variablesFactory.NewVariables(build.TeamName(), build.PipelineName())

	var taskConfigSource exec.TaskConfigSource
	var taskVars []template.Variables

	if plan.Task.ConfigPath != "" {
		// external task - construct a source which reads it from file
		taskConfigSource = exec.FileConfigSource{ConfigPath: plan.Task.ConfigPath}

		// for interpolation - use 'vars' from the pipeline, and then fill remaining with cred mgr variables
		taskVars = []template.Variables{template.StaticVariables(plan.Task.Vars), credMgrVariables}
	} else {
		// embedded task - first we take it
		taskConfigSource = exec.StaticConfigSource{Config: plan.Task.Config}

		// for interpolation - use just cred mgr variables
		taskVars = []template.Variables{credMgrVariables}
	}

	// override params
	taskConfigSource = &exec.OverrideParamsConfigSource{ConfigSource: taskConfigSource, Params: plan.Task.Params}

	// interpolate template vars
	taskConfigSource = exec.InterpolateTemplateConfigSource{ConfigSource: taskConfigSource, Vars: taskVars}

	// validate
	taskConfigSource = exec.ValidatingConfigSource{ConfigSource: taskConfigSource}

	taskStep := exec.NewTaskStep(
		exec.Privileged(plan.Task.Privileged),
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

	return exec.LogError(taskStep, delegate)
}

func (factory *stepFactory) ArtifactInputStep(
	plan atc.Plan,
	build db.Build,
	delegate exec.BuildStepDelegate,
) exec.Step {
	return exec.NewArtifactInputStep(plan, build, factory.client, delegate)
}

func (factory *stepFactory) ArtifactOutputStep(
	plan atc.Plan,
	build db.Build,
	delegate exec.BuildStepDelegate,
) exec.Step {
	return exec.NewArtifactOutputStep(plan, build, factory.client, delegate)
}
