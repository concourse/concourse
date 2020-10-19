package builder

import (
	"crypto/sha1"
	"fmt"
	"path/filepath"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/worker"
)

type stepFactory struct {
	pool                  worker.Pool
	client                worker.Client
	resourceFactory       resource.ResourceFactory
	teamFactory           db.TeamFactory
	buildFactory          db.BuildFactory
	resourceCacheFactory  db.ResourceCacheFactory
	resourceConfigFactory db.ResourceConfigFactory
	defaultLimits         atc.ContainerLimits
	strategy              worker.ContainerPlacementStrategy
	lockFactory           lock.LockFactory
	defaultCheckTimeout   time.Duration
}

func NewStepFactory(
	pool worker.Pool,
	client worker.Client,
	resourceFactory resource.ResourceFactory,
	teamFactory db.TeamFactory,
	buildFactory db.BuildFactory,
	resourceCacheFactory db.ResourceCacheFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	defaultLimits atc.ContainerLimits,
	strategy worker.ContainerPlacementStrategy,
	lockFactory lock.LockFactory,
	defaultCheckTimeout time.Duration,
) *stepFactory {
	return &stepFactory{
		pool:                  pool,
		client:                client,
		resourceFactory:       resourceFactory,
		teamFactory:           teamFactory,
		buildFactory:          buildFactory,
		resourceCacheFactory:  resourceCacheFactory,
		resourceConfigFactory: resourceConfigFactory,
		defaultLimits:         defaultLimits,
		strategy:              strategy,
		lockFactory:           lockFactory,
		defaultCheckTimeout:   defaultCheckTimeout,
	}
}

func (factory *stepFactory) GetStep(
	plan atc.Plan,
	stepMetadata exec.StepMetadata,
	containerMetadata db.ContainerMetadata,
	delegateFactory DelegateFactory,
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
		delegateFactory,
		factory.client,
	)

	getStep = exec.LogError(getStep, delegateFactory)
	if atc.EnableBuildRerunWhenWorkerDisappears {
		getStep = exec.RetryError(getStep, delegateFactory)
	}
	return getStep
}

func (factory *stepFactory) PutStep(
	plan atc.Plan,
	stepMetadata exec.StepMetadata,
	containerMetadata db.ContainerMetadata,
	delegateFactory DelegateFactory,
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
		delegateFactory,
	)

	putStep = exec.LogError(putStep, delegateFactory)
	if atc.EnableBuildRerunWhenWorkerDisappears {
		putStep = exec.RetryError(putStep, delegateFactory)
	}
	return putStep
}

func (factory *stepFactory) CheckStep(
	plan atc.Plan,
	stepMetadata exec.StepMetadata,
	containerMetadata db.ContainerMetadata,
	delegateFactory DelegateFactory,
) exec.Step {
	containerMetadata.WorkingDirectory = resource.ResourcesDir("check")
	// TODO (runtime/#4957): Placement Strategy should be abstracted out from step factory or step level concern
	checkStep := exec.NewCheckStep(
		plan.ID,
		*plan.Check,
		stepMetadata,
		factory.resourceFactory,
		factory.resourceConfigFactory,
		containerMetadata,
		worker.NewRandomPlacementStrategy(),
		factory.pool,
		delegateFactory,
		factory.client,
		factory.defaultCheckTimeout,
	)

	checkStep = exec.LogError(checkStep, delegateFactory)
	if atc.EnableBuildRerunWhenWorkerDisappears {
		checkStep = exec.RetryError(checkStep, delegateFactory)
	}
	return checkStep
}

func (factory *stepFactory) TaskStep(
	plan atc.Plan,
	stepMetadata exec.StepMetadata,
	containerMetadata db.ContainerMetadata,
	delegateFactory DelegateFactory,
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
		delegateFactory,
		factory.lockFactory,
	)

	taskStep = exec.LogError(taskStep, delegateFactory)
	if atc.EnableBuildRerunWhenWorkerDisappears {
		taskStep = exec.RetryError(taskStep, delegateFactory)
	}
	return taskStep
}

func (factory *stepFactory) SetPipelineStep(
	plan atc.Plan,
	stepMetadata exec.StepMetadata,
	delegateFactory DelegateFactory,
) exec.Step {
	spStep := exec.NewSetPipelineStep(
		plan.ID,
		*plan.SetPipeline,
		stepMetadata,
		delegateFactory,
		factory.teamFactory,
		factory.buildFactory,
		factory.client,
	)

	spStep = exec.LogError(spStep, delegateFactory)
	if atc.EnableBuildRerunWhenWorkerDisappears {
		spStep = exec.RetryError(spStep, delegateFactory)
	}
	return spStep
}

func (factory *stepFactory) LoadVarStep(
	plan atc.Plan,
	stepMetadata exec.StepMetadata,
	delegateFactory DelegateFactory,
) exec.Step {
	loadVarStep := exec.NewLoadVarStep(
		plan.ID,
		*plan.LoadVar,
		stepMetadata,
		delegateFactory,
		factory.client,
	)

	loadVarStep = exec.LogError(loadVarStep, delegateFactory)
	if atc.EnableBuildRerunWhenWorkerDisappears {
		loadVarStep = exec.RetryError(loadVarStep, delegateFactory)
	}
	return loadVarStep
}

func (factory *stepFactory) ArtifactInputStep(
	plan atc.Plan,
	build db.Build,
) exec.Step {
	return exec.NewArtifactInputStep(plan, build, factory.client)
}

func (factory *stepFactory) ArtifactOutputStep(
	plan atc.Plan,
	build db.Build,
) exec.Step {
	return exec.NewArtifactOutputStep(plan, build, factory.client)
}
