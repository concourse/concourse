package exec

import (
	"context"
	"fmt"
	"io"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/vars"
)

type ErrPipelineNotFound struct {
	PipelineName string
}

func (e ErrPipelineNotFound) Error() string {
	return fmt.Sprintf("pipeline '%s' not found", e.PipelineName)
}

type ErrResourceNotFound struct {
	ResourceName string
}

func (e ErrResourceNotFound) Error() string {
	return fmt.Sprintf("resource '%s' not found", e.ResourceName)
}

//go:generate counterfeiter . GetDelegate

type GetDelegate interface {
	ImageVersionDetermined(db.UsedResourceCache) error

	Stdout() io.Writer
	Stderr() io.Writer

	Variables() vars.CredVarsTracker

	Initializing(lager.Logger)
	Starting(lager.Logger)
	Finished(lager.Logger, ExitStatus, runtime.VersionResult)
	Errored(lager.Logger, string)

	UpdateVersion(lager.Logger, atc.GetPlan, runtime.VersionResult)
}

// GetStep will fetch a version of a resource on a worker that supports the
// resource type.
type GetStep struct {
	planID               atc.PlanID
	plan                 atc.GetPlan
	metadata             StepMetadata
	containerMetadata    db.ContainerMetadata
	resourceCacheFactory db.ResourceCacheFactory
	strategy             worker.ContainerPlacementStrategy
	workerClient         worker.Client
	workerPool           worker.Pool
	delegate             GetDelegate
	succeeded            bool
}

func NewGetStep(
	planID atc.PlanID,
	plan atc.GetPlan,
	metadata StepMetadata,
	containerMetadata db.ContainerMetadata,
	resourceCacheFactory db.ResourceCacheFactory,
	strategy worker.ContainerPlacementStrategy,
	workerPool worker.Pool,
	delegate GetDelegate,
	client worker.Client,
) Step {
	return &GetStep{
		planID:               planID,
		plan:                 plan,
		metadata:             metadata,
		containerMetadata:    containerMetadata,
		resourceCacheFactory: resourceCacheFactory,
		strategy:             strategy,
		workerPool:           workerPool,
		delegate:             delegate,
		workerClient:         client,
	}
}

// Run ultimately registers the configured resource version's StreamableArtifactSource
// under the configured SourceName. How it actually does this is determined by
// a few factors.
//
// First, a worker that supports the given resource type is chosen, and a
// container is created on the worker.
//
// If the worker has a VolumeManager, and its cache is already warmed, the
// cache will be mounted into the container, and no fetching will be performed.
// The container will be used to stream the contents of the cache to later
// steps that require the artifact but are running on a worker that does not
// have the cache.
//
// If the worker does not have a VolumeManager, or if the worker does have a
// VolumeManager but a cache for the version of the resource is not present,
// the specified version of the resource will be fetched. As long as running
// the fetch script works, Run will return nil regardless of its exit status.
//
// If the worker has a VolumeManager but did not have the cache initially, the
// fetched StreamableArtifactSource is initialized, thus warming the worker's cache.
//
// At the end, the resulting StreamableArtifactSource (either from using the cache or
// fetching the resource) is registered under the step's SourceName.
func (step *GetStep) Run(ctx context.Context, state RunState) error {
	logger := lagerctx.FromContext(ctx)
	logger = logger.Session("get-step", lager.Data{
		"step-name": step.plan.Name,
		"job-id":    step.metadata.JobID,
	})

	step.delegate.Initializing(logger)

	variables := step.delegate.Variables()

	source, err := creds.NewSource(variables, step.plan.Source).Evaluate()
	if err != nil {
		return err
	}

	params, err := creds.NewParams(variables, step.plan.Params).Evaluate()
	if err != nil {
		return err
	}

	resourceTypes, err := creds.NewVersionedResourceTypes(variables, step.plan.VersionedResourceTypes).Evaluate()
	if err != nil {
		return err
	}

	version, err := NewVersionSourceFromPlan(&step.plan).Version(state)
	if err != nil {
		return err
	}

	containerSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: step.plan.Type,
		},
		TeamID: step.metadata.TeamID,
		Env:    step.metadata.Env(),
	}

	workerSpec := worker.WorkerSpec{
		ResourceType:  step.plan.Type,
		Tags:          step.plan.Tags,
		TeamID:        step.metadata.TeamID,
		ResourceTypes: resourceTypes,
	}

	resourceCache, err := step.resourceCacheFactory.FindOrCreateResourceCache(
		db.ForBuild(step.metadata.BuildID),
		step.plan.Type,
		version,
		source,
		params,
		resourceTypes,
	)
	if err != nil {
		logger.Error("failed-to-create-resource-cache", err)
		return err
	}

	// TODO containerOwner accepts workerName and this should be extracted out
	// Stuff above is all part of Concourse CORE

	events := make(chan runtime.Event, 1)
	go func(logger lager.Logger, events chan runtime.Event, delegate GetDelegate) {
		for {
			ev := <-events
			switch {
			case ev.EventType == runtime.InitializingEvent:
				step.delegate.Initializing(logger)

			case ev.EventType == runtime.StartingEvent:
				step.delegate.Starting(logger)

			case ev.EventType == runtime.FinishedEvent:
				step.delegate.Finished(logger, ExitStatus(ev.ExitStatus), ev.VersionResult)

			default:
				return
			}
		}
	}(logger, events, step.delegate)

	resourceDir := resource.ResourcesDir("get")
	processSpec := runtime.ProcessSpec{
		Path:         "/opt/resource/in",
		Args:         []string{resourceDir},
		StdoutWriter: step.delegate.Stdout(),
		StderrWriter: step.delegate.Stderr(),
	}

	res := resource.NewResource(
		source,
		params,
		version,
	)

	// start of workerClient.RunGetStep?
	getResult, err := step.workerClient.RunGetStep(
		ctx,
		logger,
		db.NewBuildStepContainerOwner(step.metadata.BuildID, step.planID, step.metadata.TeamID),
		containerSpec,
		workerSpec,
		step.strategy,
		step.containerMetadata,
		resourceTypes,
		//source,
		//params,
		res,
		resourceDir,
		step.delegate,
		resourceCache,
		processSpec,
		events,
	)

	if err != nil {
		return err
	}

	if getResult.Status == 0 {
		// TODO move all the state changing logic from resourceInstnanceFetchSource.Create to here
		//err = volume.InitializeResourceCache(s.resourceInstance.ResourceCache())
		//if err != nil {
		//	sLog.Error("failed-to-initialize-cache", err)
		//	return nil, err
		//}
		//
		//err = s.dbResourceCacheFactory.UpdateResourceCacheMetadata(s.resourceInstance.ResourceCache(), versionedSource.Metadata())
		//if err != nil {
		//	s.logger.Error("failed-to-update-resource-cache-metadata", err, lager.Data{"resource-cache": s.resourceInstance.ResourceCache()})
		//	return nil, err
		//}

		state.ArtifactRepository().RegisterArtifact(build.ArtifactName(step.plan.Name), getResult.GetArtifact)

		if step.plan.Resource != "" {
			step.delegate.UpdateVersion(logger, step.plan, getResult.VersionResult)
		}

		step.delegate.Finished(logger, 0, getResult.VersionResult)

		step.succeeded = true
	} else {
		// TODO  have a way of bubbling up the error message from getResult.Err
		step.delegate.Finished(logger, ExitStatus(getResult.Status), getResult.VersionResult)
	}

	return nil
}

// Succeeded returns true if the resource was successfully fetched.
func (step *GetStep) Succeeded() bool {
	return step.succeeded
}
