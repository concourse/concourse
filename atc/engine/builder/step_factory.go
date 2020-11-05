package builder

import (
	"crypto/sha1"
	"fmt"
	"path/filepath"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/worker"
)

type stepFactory struct {
	pool                            worker.Pool
	client                          worker.Client
	resourceFactory                 resource.ResourceFactory
	teamFactory                     db.TeamFactory
	resourceCacheFactory            db.ResourceCacheFactory
	resourceConfigFactory           db.ResourceConfigFactory
	defaultLimits                   atc.ContainerLimits
	strategy                        worker.ContainerPlacementStrategy
	lockFactory                     lock.LockFactory
	enableRerunWhenWorkerDisappears bool
}

func NewStepFactory(
	pool worker.Pool,
	client worker.Client,
	resourceFactory resource.ResourceFactory,
	teamFactory db.TeamFactory,
	resourceCacheFactory db.ResourceCacheFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	defaultLimits atc.ContainerLimits,
	strategy worker.ContainerPlacementStrategy,
	lockFactory lock.LockFactory,
	enableRerunWhenWorkerDisappears bool,
) *stepFactory {
	return &stepFactory{
		pool:                            pool,
		client:                          client,
		resourceFactory:                 resourceFactory,
		teamFactory:                     teamFactory,
		resourceCacheFactory:            resourceCacheFactory,
		resourceConfigFactory:           resourceConfigFactory,
		defaultLimits:                   defaultLimits,
		strategy:                        strategy,
		lockFactory:                     lockFactory,
		enableRerunWhenWorkerDisappears: enableRerunWhenWorkerDisappears,
	}
}

func (factory *stepFactory) GetStep(
	plan atc.Plan,
	stepMetadata exec.StepMetadata,
	containerMetadata db.ContainerMetadata,
	delegate exec.GetDelegate,
) exec.Step {
	containerMetadata.WorkingDirectory = resource.ResourcesDir("get")

	getStep := exec.NewGetStep(
		plan.ID,
		*plan.Get,
		stepMetadata,
		containerMetadata,
		factory.resourceFactory,
		factory.resourceCacheFactory,
		factory.strategy,
		delegate,
		factory.client,
	)

	getStep = exec.LogError(getStep, delegate)
	if factory.enableRerunWhenWorkerDisappears {
		getStep = exec.RetryError(getStep, delegate)
	}
	return getStep
}

func (factory *stepFactory) PutStep(
	plan atc.Plan,
	stepMetadata exec.StepMetadata,
	containerMetadata db.ContainerMetadata,
	delegate exec.PutDelegate,
) exec.Step {
	containerMetadata.WorkingDirectory = resource.ResourcesDir("put")

	putStep := exec.NewPutStep(
		plan.ID,
		*plan.Put,
		stepMetadata,
		containerMetadata,
		factory.resourceFactory,
		factory.resourceConfigFactory,
		factory.strategy,
		factory.client,
		delegate,
	)

	putStep = exec.LogError(putStep, delegate)
	if factory.enableRerunWhenWorkerDisappears {
		putStep = exec.RetryError(putStep, delegate)
	}
	return putStep
}

func (factory *stepFactory) CheckStep(
	plan atc.Plan,
	stepMetadata exec.StepMetadata,
	containerMetadata db.ContainerMetadata,
	delegate exec.CheckDelegate,
) exec.Step {
	containerMetadata.WorkingDirectory = resource.ResourcesDir("check")
	// TODO (runtime/#4957): Placement Strategy should be abstracted out from step factory or step level concern
	checkStep := exec.NewCheckStep(
		plan.ID,
		*plan.Check,
		stepMetadata,
		factory.resourceFactory,
		containerMetadata,
		worker.NewRandomPlacementStrategy(),
		factory.pool,
		delegate,
		factory.client,
	)

	return checkStep
}

func (factory *stepFactory) TaskStep(
	plan atc.Plan,
	stepMetadata exec.StepMetadata,
	containerMetadata db.ContainerMetadata,
	delegate exec.TaskDelegate,
	workerOverrides atc.UnsafeWorkerOverrides,
) exec.Step {
	sum := sha1.Sum([]byte(plan.Task.Name))
	containerMetadata.WorkingDirectory = filepath.Join("/tmp", "build", fmt.Sprintf("%x", sum[:4]))

	taskStep := exec.NewTaskStep(
		plan.ID,
		*plan.Task,
		factory.defaultLimits,
		stepMetadata,
		containerMetadata,
		factory.strategy,
		factory.client,
		delegate,
		factory.lockFactory,
		workerOverrides,
	)

	taskStep = exec.LogError(taskStep, delegate)
	if factory.enableRerunWhenWorkerDisappears {
		taskStep = exec.RetryError(taskStep, delegate)
	}
	return taskStep
}

func (factory *stepFactory) SetPipelineStep(
	plan atc.Plan,
	stepMetadata exec.StepMetadata,
	delegate exec.BuildStepDelegate,
) exec.Step {
	spStep := exec.NewSetPipelineStep(
		plan.ID,
		*plan.SetPipeline,
		stepMetadata,
		delegate,
		factory.teamFactory,
		factory.client,
	)

	spStep = exec.LogError(spStep, delegate)
	if factory.enableRerunWhenWorkerDisappears {
		spStep = exec.RetryError(spStep, delegate)
	}
	return spStep
}

func (factory *stepFactory) LoadVarStep(
	plan atc.Plan,
	stepMetadata exec.StepMetadata,
	delegate exec.BuildStepDelegate,
) exec.Step {
	loadVarStep := exec.NewLoadVarStep(
		plan.ID,
		*plan.LoadVar,
		stepMetadata,
		delegate,
		factory.client,
	)

	loadVarStep = exec.LogError(loadVarStep, delegate)
	if factory.enableRerunWhenWorkerDisappears {
		loadVarStep = exec.RetryError(loadVarStep, delegate)
	}
	return loadVarStep
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
