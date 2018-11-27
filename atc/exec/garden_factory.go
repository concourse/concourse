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
	workerClient          worker.Client
	resourceFetcher       resource.Fetcher
	resourceFactory       resource.ResourceFactory
	resourceCacheFactory  db.ResourceCacheFactory
	resourceConfigFactory db.ResourceConfigFactory
	variablesFactory      creds.VariablesFactory
	defaultLimits         atc.ContainerLimits
}

func NewGardenFactory(
	workerClient worker.Client,
	resourceFetcher resource.Fetcher,
	resourceFactory resource.ResourceFactory,
	resourceCacheFactory db.ResourceCacheFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	variablesFactory creds.VariablesFactory,
	defaultLimits atc.ContainerLimits,
) Factory {
	return &gardenFactory{
		workerClient:          workerClient,
		resourceFetcher:       resourceFetcher,
		resourceFactory:       resourceFactory,
		resourceCacheFactory:  resourceCacheFactory,
		resourceConfigFactory: resourceConfigFactory,
		variablesFactory:      variablesFactory,
		defaultLimits:         defaultLimits,
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
	if plan.Put.Inputs != nil {
		putInputs = NewSpecificInputs(plan.Put.Inputs)
	} else {
		putInputs = NewAllInputs()
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
		factory.resourceFactory,
		factory.resourceConfigFactory,
		plan.ID,
		workerMetadata,
		stepMetadata,

		creds.NewVersionedResourceTypes(variables, plan.Put.VersionedResourceTypes),
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
	if plan.Task.ConfigPath != "" {
		// external task - interpolate it with vars + cred mgr variables
		taskVars := []boshtemplate.Variables{boshtemplate.StaticVariables(plan.Task.Vars), credMgrVariables}
		taskConfigSource = FileConfigSource{ConfigPath: plan.Task.ConfigPath, Vars: taskVars}
	} else {
		// embedded task - interpolate it with just cred mgr variables
		taskVars := []boshtemplate.Variables{credMgrVariables}
		taskConfigSource = StaticConfigSource{Config: plan.Task.Config, Vars: taskVars}
	}

	// override params
	taskConfigSource = OverrideParamsConfigSource{ConfigSource: taskConfigSource, Params: plan.Task.Params}

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

		factory.workerClient,
		build.TeamID(),
		build.ID(),
		build.JobID(),
		plan.Task.Name,
		plan.ID,
		containerMetadata,

		creds.NewVersionedResourceTypes(credMgrVariables, plan.Task.VersionedResourceTypes),
		factory.defaultLimits,
	)

	return LogError(taskStep, delegate)
}

func (factory *gardenFactory) taskWorkingDirectory(sourceName worker.ArtifactName) string {
	sum := sha1.Sum([]byte(sourceName))
	return filepath.Join("/tmp", "build", fmt.Sprintf("%x", sum[:4]))
}
