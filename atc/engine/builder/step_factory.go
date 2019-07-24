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
	pool                  worker.Pool
	client                worker.Client
	resourceFetcher       worker.Fetcher
	teamFactory           db.TeamFactory
	resourceCacheFactory  db.ResourceCacheFactory
	resourceConfigFactory db.ResourceConfigFactory
	defaultLimits         atc.ContainerLimits
	strategy              worker.ContainerPlacementStrategy
	resourceFactory       resource.ResourceFactory
	lockFactory           lock.LockFactory
}

func NewStepFactory(
	pool worker.Pool,
	client worker.Client,
	resourceFetcher worker.Fetcher,
	teamFactory db.TeamFactory,
	resourceCacheFactory db.ResourceCacheFactory,
	resourceConfigFactory db.ResourceConfigFactory,
	defaultLimits atc.ContainerLimits,
	strategy worker.ContainerPlacementStrategy,
	resourceFactory resource.ResourceFactory,
	lockFactory lock.LockFactory,
) *stepFactory {
	return &stepFactory{
		pool:                  pool,
		client:                client,
		resourceFetcher:       resourceFetcher,
		teamFactory:           teamFactory,
		resourceCacheFactory:  resourceCacheFactory,
		resourceConfigFactory: resourceConfigFactory,
		defaultLimits:         defaultLimits,
		strategy:              strategy,
		resourceFactory:       resourceFactory,
		lockFactory:           lockFactory,
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
		factory.resourceFetcher,
		factory.resourceCacheFactory,
		factory.strategy,
		factory.pool,
		delegate,
	)

	return exec.LogError(getStep, delegate)
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

	return exec.LogError(putStep, delegate)
}

func (factory *stepFactory) CheckStep(
	plan atc.Plan,
	stepMetadata exec.StepMetadata,
	containerMetadata db.ContainerMetadata,
	delegate exec.CheckDelegate,
) exec.Step {
	containerMetadata.WorkingDirectory = resource.ResourcesDir("check")

	checkStep := exec.NewCheckStep(
		plan.ID,
		*plan.Check,
		stepMetadata,
		containerMetadata,
		factory.resourceFactory,
		worker.NewRandomPlacementStrategy(),
		factory.pool,
		delegate,
	)

	return checkStep
}

func (factory *stepFactory) TaskStep(
	plan atc.Plan,
	stepMetadata exec.StepMetadata,
	containerMetadata db.ContainerMetadata,
	delegate exec.TaskDelegate,
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
	)

	return exec.LogError(taskStep, delegate)
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
	)

	return exec.LogError(spStep, delegate)
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
