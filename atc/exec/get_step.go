package exec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/tracing"
	"go.opentelemetry.io/otel/trace"
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

//counterfeiter:generate . GetDelegateFactory
type GetDelegateFactory interface {
	GetDelegate(state RunState) GetDelegate
}

//counterfeiter:generate . GetDelegate
type GetDelegate interface {
	StartSpan(context.Context, string, tracing.Attrs) (context.Context, trace.Span)

	FetchImage(context.Context, atc.ImageResource, atc.VersionedResourceTypes, bool) (worker.ImageSpec, error)

	Stdout() io.Writer
	Stderr() io.Writer

	Initializing(lager.Logger)
	Starting(lager.Logger)
	Finished(lager.Logger, ExitStatus, runtime.VersionResult)
	Errored(lager.Logger, string)

	WaitingForWorker(lager.Logger)
	SelectedWorker(lager.Logger, string)

	UpdateVersion(lager.Logger, atc.GetPlan, runtime.VersionResult)

	ResourceCacheUser() db.ResourceCacheUser
	ContainerOwner(planId atc.PlanID) db.ContainerOwner
}

// GetStep will fetch a version of a resource on a worker that supports the
// resource type.
type GetStep struct {
	planID               atc.PlanID
	plan                 atc.GetPlan
	metadata             StepMetadata
	containerMetadata    db.ContainerMetadata
	resourceFactory      resource.ResourceFactory
	resourceCacheFactory db.ResourceCacheFactory
	strategy             worker.ContainerPlacementStrategy
	workerPool           worker.Pool
	delegateFactory      GetDelegateFactory
}

func NewGetStep(
	planID atc.PlanID,
	plan atc.GetPlan,
	metadata StepMetadata,
	containerMetadata db.ContainerMetadata,
	resourceFactory resource.ResourceFactory,
	resourceCacheFactory db.ResourceCacheFactory,
	strategy worker.ContainerPlacementStrategy,
	delegateFactory GetDelegateFactory,
	pool worker.Pool,
) Step {
	return &GetStep{
		planID:               planID,
		plan:                 plan,
		metadata:             metadata,
		containerMetadata:    containerMetadata,
		resourceFactory:      resourceFactory,
		resourceCacheFactory: resourceCacheFactory,
		strategy:             strategy,
		delegateFactory:      delegateFactory,
		workerPool:           pool,
	}
}

func (step *GetStep) Run(ctx context.Context, state RunState) (bool, error) {
	delegate := step.delegateFactory.GetDelegate(state)
	ctx, span := delegate.StartSpan(ctx, "get", tracing.Attrs{
		"name":     step.plan.Name,
		"resource": step.plan.Resource,
	})

	ok, err := step.run(ctx, state, delegate)
	tracing.End(span, err)

	return ok, err
}

func (step *GetStep) run(ctx context.Context, state RunState, delegate GetDelegate) (bool, error) {
	logger := lagerctx.FromContext(ctx)
	logger = logger.Session("get-step", lager.Data{
		"step-name": step.plan.Name,
	})

	delegate.Initializing(logger)

	source, err := creds.NewSource(state, step.plan.Source).Evaluate()
	if err != nil {
		return false, err
	}

	params, err := creds.NewParams(state, step.plan.Params).Evaluate()
	if err != nil {
		return false, err
	}

	workerSpec := worker.WorkerSpec{
		Tags:         step.plan.Tags,
		TeamID:       step.metadata.TeamID,
		ResourceType: step.plan.VersionedResourceTypes.Base(step.plan.Type),
	}

	resourceTypes, err := creds.NewVersionedResourceTypes(state, step.plan.VersionedResourceTypes).Evaluate()
	if err != nil {
		return false, err
	}

	version, err := NewVersionSourceFromPlan(&step.plan).Version(state)
	if err != nil {
		return false, err
	}

	resourceCache, err := step.resourceCacheFactory.FindOrCreateResourceCache(
		delegate.ResourceCacheUser(),
		step.plan.Type,
		version,
		source,
		params,
		resourceTypes,
	)
	if err != nil {
		logger.Error("failed-to-create-resource-cache", err)
		return false, err
	}

	// Only get from local cache if caching streamed volumes is enabled -
	// otherwise, we'd need to stream volumes between workers much more
	// frequently.
	if atc.EnableCacheStreamedVolumes {
		getResult, found, err := step.getFromLocalCache(logger, step.metadata.TeamID, resourceCache, workerSpec)
		if err != nil {
			return false, err
		}
		if found {
			fmt.Fprintln(delegate.Stderr(), "\x1b[1;36mINFO: found resource cache from local cache\x1b[0m")
			fmt.Fprintln(delegate.Stderr(), "")

			delegate.Starting(logger)
			state.StoreResult(step.planID, resourceCache)

			state.ArtifactRepository().RegisterArtifact(
				build.ArtifactName(step.plan.Name),
				getResult.GetArtifact,
			)

			if step.plan.Resource != "" {
				delegate.UpdateVersion(logger, step.plan, getResult.VersionResult)
			}

			delegate.Finished(
				logger,
				ExitStatus(getResult.ExitStatus),
				getResult.VersionResult,
			)

			metric.Metrics.GetStepCacheHits.Inc()

			return true, nil
		}
	}

	var imageSpec worker.ImageSpec
	resourceType, found := step.plan.VersionedResourceTypes.Lookup(step.plan.Type)
	if found {
		image := atc.ImageResource{
			Name:    resourceType.Name,
			Type:    resourceType.Type,
			Source:  resourceType.Source,
			Params:  resourceType.Params,
			Version: resourceType.Version,
			Tags:    resourceType.Tags,
		}
		if len(image.Tags) == 0 {
			image.Tags = step.plan.Tags
		}

		types := step.plan.VersionedResourceTypes.Without(step.plan.Type)

		var err error
		imageSpec, err = delegate.FetchImage(ctx, image, types, resourceType.Privileged)
		if err != nil {
			return false, err
		}
	} else {
		imageSpec.ResourceType = step.plan.Type
	}

	containerSpec := worker.ContainerSpec{
		ImageSpec: imageSpec,
		TeamID:    step.metadata.TeamID,
		TeamName:  step.metadata.TeamName,
		Type:      step.containerMetadata.Type,

		Env: step.metadata.Env(),
	}
	tracing.Inject(ctx, &containerSpec)

	processSpec := runtime.ProcessSpec{
		Path:         "/opt/resource/in",
		Args:         []string{resource.ResourcesDir("get")},
		StdoutWriter: delegate.Stdout(),
		StderrWriter: delegate.Stderr(),
	}

	resourceToGet := step.resourceFactory.NewResource(
		source,
		params,
		version,
	)

	containerOwner := delegate.ContainerOwner(step.planID)

	worker, _, err := step.workerPool.SelectWorker(
		lagerctx.NewContext(ctx, logger),
		containerOwner,
		containerSpec,
		workerSpec,
		step.strategy,
		delegate,
	)
	if err != nil {
		return false, err
	}

	delegate.SelectedWorker(logger, worker.Name())

	defer func() {
		step.workerPool.ReleaseWorker(
			lagerctx.NewContext(ctx, logger),
			containerSpec,
			worker,
			step.strategy,
		)
	}()

	processCtx, cancel, err := MaybeTimeout(ctx, step.plan.Timeout)
	if err != nil {
		return false, err
	}

	defer cancel()

	getResult, err := worker.RunGetStep(
		lagerctx.NewContext(processCtx, logger),
		containerOwner,
		containerSpec,
		step.containerMetadata,
		processSpec,
		delegate,
		resourceCache,
		resourceToGet,
	)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			delegate.Errored(logger, TimeoutLogMessage)
			return false, nil
		}

		return false, err
	}

	var succeeded bool
	if getResult.ExitStatus == 0 {
		state.StoreResult(step.planID, resourceCache)

		state.ArtifactRepository().RegisterArtifact(
			build.ArtifactName(step.plan.Name),
			getResult.GetArtifact,
		)

		if step.plan.Resource != "" {
			delegate.UpdateVersion(logger, step.plan, getResult.VersionResult)
		}

		succeeded = true
	}

	delegate.Finished(
		logger,
		ExitStatus(getResult.ExitStatus),
		getResult.VersionResult,
	)

	return succeeded, nil
}

func (step *GetStep) getFromLocalCache(
	logger lager.Logger,
	teamId int,
	resourceCache db.UsedResourceCache,
	workerSpec worker.WorkerSpec) (worker.GetResult, bool, error) {
	volume, found := step.findResourceCache(logger, teamId, resourceCache, workerSpec)
	if !found {
		return worker.GetResult{}, false, nil
	}
	metadata, err := step.resourceCacheFactory.ResourceCacheMetadata(resourceCache)
	if err != nil {
		return worker.GetResult{}, false, err
	}
	return worker.GetResult{
		ExitStatus: 0,
		VersionResult: runtime.VersionResult{
			Version:  resourceCache.Version(),
			Metadata: metadata.ToATCMetadata(),
		},
		GetArtifact: runtime.GetArtifact{volume.Handle()},
	}, true, nil
}

func (step *GetStep) findResourceCache(
	logger lager.Logger,
	teamId int,
	resourceCache db.UsedResourceCache,
	workerSpec worker.WorkerSpec) (worker.Volume, bool) {
	workers, err := step.workerPool.FindWorkersForResourceCache(logger, teamId, resourceCache.ID(), workerSpec)
	if err != nil {
		return nil, false
	}

	// Randomize worker order so that the same worker doesn't have to perform
	// the streaming constantly
	rand.Shuffle(len(workers), func(i, j int) {
		workers[i], workers[j] = workers[j], workers[i]
	})

	for _, sourceWorker := range workers {
		volume, found, err := sourceWorker.FindVolumeForResourceCache(logger, resourceCache)
		if err != nil {
			logger.Debug("ignore-find-volume-for-resource-cache-error",
				lager.Data{
					"error":           err,
					"sourceWorker":    sourceWorker,
					"resourceCacheId": resourceCache.ID(),
				})
			continue
		}
		if !found {
			continue
		}
		return volume, true
	}

	return nil, false
}
