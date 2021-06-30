package engine

import (
	"crypto/sha1"
	"fmt"
	"path/filepath"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/worker"
)

type coreStepFactory struct {
	pool                  worker.Pool
	artifactStreamer      worker.ArtifactStreamer
	artifactSourcer       worker.ArtifactSourcer
	resourceFactory       resource.ResourceFactory
	teamFactory           db.TeamFactory
	buildFactory          db.BuildFactory
	resourceCacheFactory  db.ResourceCacheFactory
	resourceConfigFactory db.ResourceConfigFactory
	defaultLimits         atc.ContainerLimits
	strategy              worker.ContainerPlacementStrategy
	defaultCheckTimeout   time.Duration
}

func NewCoreStepFactory(
	pool worker.Pool,
	artifactStreamer worker.ArtifactStreamer,
	artifactSourcer worker.ArtifactSourcer,
	resourceFactory resource.ResourceFactory,
	teamFactory db.TeamFactory,
	buildFactory db.BuildFactory,
	resourceCacheFactory db.ResourceCacheFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	defaultLimits atc.ContainerLimits,
	strategy worker.ContainerPlacementStrategy,
	defaultCheckTimeout time.Duration,
) CoreStepFactory {
	return &coreStepFactory{
		pool:                  pool,
		artifactStreamer:      artifactStreamer,
		artifactSourcer:       artifactSourcer,
		resourceFactory:       resourceFactory,
		teamFactory:           teamFactory,
		buildFactory:          buildFactory,
		resourceCacheFactory:  resourceCacheFactory,
		resourceConfigFactory: resourceConfigFactory,
		defaultLimits:         defaultLimits,
		strategy:              strategy,
		defaultCheckTimeout:   defaultCheckTimeout,
	}
}

func (factory *coreStepFactory) GetStep(
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
		factory.pool,
	)

	getStep = exec.LogError(getStep, delegateFactory)
	if atc.EnableBuildRerunWhenWorkerDisappears {
		getStep = exec.RetryError(getStep, delegateFactory)
	}
	return getStep
}

func (factory *coreStepFactory) PutStep(
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
		factory.pool,
		factory.artifactSourcer,
		delegateFactory,
	)

	putStep = exec.LogError(putStep, delegateFactory)
	if atc.EnableBuildRerunWhenWorkerDisappears {
		putStep = exec.RetryError(putStep, delegateFactory)
	}
	return putStep
}

func (factory *coreStepFactory) CheckStep(
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
		factory.defaultCheckTimeout,
	)

	checkStep = exec.LogError(checkStep, delegateFactory)
	if atc.EnableBuildRerunWhenWorkerDisappears {
		checkStep = exec.RetryError(checkStep, delegateFactory)
	}
	return checkStep
}

func (factory *coreStepFactory) RunStep(
	plan atc.Plan,
	stepMetadata exec.StepMetadata,
	containerMetadata db.ContainerMetadata,
	delegateFactory DelegateFactory,
) exec.Step {
	containerMetadata.WorkingDirectory = "/tmp/build/run"

	runStep := exec.NewRunStep(
		plan.ID,
		*plan.Run,
		delegateFactory,
	)

	runStep = exec.LogError(runStep, delegateFactory)
	if atc.EnableBuildRerunWhenWorkerDisappears {
		runStep = exec.RetryError(runStep, delegateFactory)
	}
	return runStep
}

func (factory *coreStepFactory) TaskStep(
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
		factory.pool,
		factory.artifactStreamer,
		factory.artifactSourcer,
		delegateFactory,
	)

	taskStep = exec.LogError(taskStep, delegateFactory)
	if atc.EnableBuildRerunWhenWorkerDisappears {
		taskStep = exec.RetryError(taskStep, delegateFactory)
	}
	return taskStep
}

func (factory *coreStepFactory) SetPipelineStep(
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
		factory.artifactStreamer,
	)

	spStep = exec.LogError(spStep, delegateFactory)
	if atc.EnableBuildRerunWhenWorkerDisappears {
		spStep = exec.RetryError(spStep, delegateFactory)
	}
	return spStep
}

func (factory *coreStepFactory) LoadVarStep(
	plan atc.Plan,
	stepMetadata exec.StepMetadata,
	delegateFactory DelegateFactory,
) exec.Step {
	loadVarStep := exec.NewLoadVarStep(
		plan.ID,
		*plan.LoadVar,
		stepMetadata,
		delegateFactory,
		factory.artifactStreamer,
	)

	loadVarStep = exec.LogError(loadVarStep, delegateFactory)
	if atc.EnableBuildRerunWhenWorkerDisappears {
		loadVarStep = exec.RetryError(loadVarStep, delegateFactory)
	}
	return loadVarStep
}

func (factory *coreStepFactory) ArtifactInputStep(
	plan atc.Plan,
	build db.Build,
) exec.Step {
	return exec.NewArtifactInputStep(plan, build, factory.pool)
}

func (factory *coreStepFactory) ArtifactOutputStep(
	plan atc.Plan,
	build db.Build,
) exec.Step {
	return exec.NewArtifactOutputStep(plan, build, factory.pool)
}
