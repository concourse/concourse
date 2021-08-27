package exec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/tracing"
	"go.opentelemetry.io/otel/trace"
)

var GetResourceLockInterval = 5 * time.Second

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

type GetResult struct {
	Name          string
	ResourceCache db.ResourceCache
}

//go:generate counterfeiter . GetDelegateFactory
type GetDelegateFactory interface {
	GetDelegate(state RunState) GetDelegate
}

//counterfeiter:generate . GetDelegate
type GetDelegate interface {
	StartSpan(context.Context, string, tracing.Attrs) (context.Context, trace.Span)

	FetchImage(context.Context, atc.Plan, *atc.Plan, bool) (runtime.ImageSpec, db.ResourceCache, error)

	Stdout() io.Writer
	Stderr() io.Writer

	Initializing(lager.Logger)
	Starting(lager.Logger)
	Finished(lager.Logger, ExitStatus, resource.VersionResult)
	Errored(lager.Logger, string)

	WaitingForWorker(lager.Logger)
	SelectedWorker(lager.Logger, string)

	UpdateResourceVersion(lager.Logger, string, resource.VersionResult)
}

// GetStep will fetch a version of a resource on a worker that supports the
// resource type.
type GetStep struct {
	planID               atc.PlanID
	plan                 atc.GetPlan
	metadata             StepMetadata
	containerMetadata    db.ContainerMetadata
	resourceCacheFactory db.ResourceCacheFactory
	strategy             worker.PlacementStrategy
	workerPool           Pool
	lockFactory          lock.LockFactory
	delegateFactory      GetDelegateFactory
}

func NewGetStep(
	planID atc.PlanID,
	plan atc.GetPlan,
	metadata StepMetadata,
	containerMetadata db.ContainerMetadata,
	lockFactory lock.LockFactory,
	resourceCacheFactory db.ResourceCacheFactory,
	strategy worker.PlacementStrategy,
	delegateFactory GetDelegateFactory,
	pool Pool,
) Step {
	return &GetStep{
		planID:               planID,
		plan:                 plan,
		metadata:             metadata,
		containerMetadata:    containerMetadata,
		resourceCacheFactory: resourceCacheFactory,
		strategy:             strategy,
		lockFactory:          lockFactory,
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

	workerSpec := worker.Spec{
		Tags:   step.plan.Tags,
		TeamID: step.metadata.TeamID,

		// Used to filter out non-Linux workers, simply because they don't support
		// base resource types
		ResourceType: step.plan.TypeImage.BaseType,
	}

	var (
		imageSpec          runtime.ImageSpec
		imageResourceCache db.ResourceCache
	)
	if step.plan.TypeImage.GetPlan != nil {
		var err error
		imageSpec, imageResourceCache, err = delegate.FetchImage(ctx, *step.plan.TypeImage.GetPlan, step.plan.TypeImage.CheckPlan, step.plan.TypeImage.Privileged)
		if err != nil {
			return false, err
		}
	} else {
		imageSpec.ResourceType = step.plan.TypeImage.BaseType
	}

	version, err := NewVersionSourceFromPlan(&step.plan).Version(state)
	if err != nil {
		return false, err
	}

	containerSpec := runtime.ContainerSpec{
		TeamID:   step.metadata.TeamID,
		TeamName: step.metadata.TeamName,
		JobID:    step.metadata.JobID,

		ImageSpec: imageSpec,

		Env:  step.metadata.Env(),
		Type: db.ContainerTypeGet,

		Dir: resource.ResourcesDir("get"),

		CertsBindMount: true,
	}
	tracing.Inject(ctx, &containerSpec)

	resourceCache, err := step.resourceCacheFactory.FindOrCreateResourceCache(
		db.ForBuild(step.metadata.BuildID),
		step.plan.Type,
		version,
		source,
		params,
		imageResourceCache,
	)
	if err != nil {
		logger.Error("failed-to-create-resource-cache", err)
		return false, err
	}

	containerOwner := db.NewBuildStepContainerOwner(step.metadata.BuildID, step.planID, step.metadata.TeamID)

	delegate.Starting(logger)
	volume, versionResult, processResult, err := step.retrieveFromCacheOrPerformGet(
		ctx,
		logger,
		delegate,
		resourceCache,
		resource.Resource{
			Source:  source,
			Params:  params,
			Version: version,
		},
		workerSpec,
		containerSpec,
		containerOwner,
	)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			delegate.Errored(logger, TimeoutLogMessage)
			return false, nil
		}

		return false, err
	}

	var succeeded bool
	if processResult.ExitStatus == 0 {
		state.StoreResult(step.planID, GetResult{
			Name:          step.plan.Name,
			ResourceCache: resourceCache,
		})

		state.ArtifactRepository().RegisterArtifact(
			build.ArtifactName(step.plan.Name),
			volume,
		)

		if step.plan.Resource != "" {
			delegate.UpdateResourceVersion(logger, step.plan.Resource, versionResult)
		}

		succeeded = true
	}

	delegate.Finished(
		logger,
		ExitStatus(processResult.ExitStatus),
		versionResult,
	)

	return succeeded, nil
}

func (step *GetStep) retrieveFromCacheOrPerformGet(
	ctx context.Context,
	logger lager.Logger,
	delegate GetDelegate,
	resourceCache db.ResourceCache,
	getResource resource.Resource,
	workerSpec worker.Spec,
	containerSpec runtime.ContainerSpec,
	containerOwner db.ContainerOwner,
) (runtime.Volume, resource.VersionResult, runtime.ProcessResult, error) {
	var worker runtime.Worker

	lockName := strconv.Itoa(resourceCache.ID())

	// If caching streamed volumes is enabled, we may be able to use a cached
	// result from another worker, so don't bother selecting a worker just yet.
	// Note that if it's disabled, we don't use cached volumes from other
	// workers so that we can hydrate the cache throughout the cluster -
	// otherwise, the few workers with the resource cache may need to perform a
	// ton of streaming out.
	if !atc.EnableCacheStreamedVolumes {
		var err error
		worker, err = step.workerPool.FindOrSelectWorker(ctx, containerOwner, containerSpec, workerSpec, step.strategy, delegate)
		if err != nil {
			logger.Error("failed-to-select-worker", err)
			return nil, resource.VersionResult{}, runtime.ProcessResult{}, err
		}

		// The lock is unique only to the current worker when not caching
		// streamed volumes since we only consider the current worker's local
		// resource cache in this case. When caching streamed volumes is
		// enabled, we consider resource caches from any (compatible) worker.
		lockName += "-" + worker.Name()

		delegate.SelectedWorker(logger, worker.Name())

		defer func() {
			step.workerPool.ReleaseWorker(
				logger,
				containerSpec,
				worker,
				step.strategy,
			)
		}()
	}

	// attemptGet performs the following flow:
	//
	// * Check if resource is cached
	//     * If yes, then use the cache and exit
	//     * If no, then proceed to next step
	// * Attempt to acquire a lock that's unique to the resource (and possibly
	//   also the worker, if EnableCacheStreamedVolumes is disabled)
	//     * If lock acquisition failed, give up (and try again after
	//       GetResourceLockInterval)
	//     * If lock acquisition succeeded, then run the get script and
	//       initialize the volume as a resource cache.
	attemptGet := func() (runtime.Volume, resource.VersionResult, runtime.ProcessResult, bool, error) {
		volume, versionResult, found, err := step.retrieveFromCache(logger, resourceCache, workerSpec, worker)
		if err != nil {
			return volume, resource.VersionResult{}, runtime.ProcessResult{}, false, err
		}
		if found {
			metric.Metrics.GetStepCacheHits.Inc()
			fmt.Fprintln(delegate.Stderr(), "\x1b[1;36mINFO: found existing resource cache\x1b[0m")
			fmt.Fprintln(delegate.Stderr(), "")
			return volume, versionResult, runtime.ProcessResult{ExitStatus: 0}, true, nil
		}

		lockLogger := logger.Session("lock", lager.Data{"lock-name": lockName})
		lock, acquired, err := step.lockFactory.Acquire(lockLogger, lock.NewTaskLockID(lockName))
		if err != nil {
			lockLogger.Error("failed-to-get-lock", err)
			// not returning error for consistency with prior behaviour - we just
			// retry after GetResourceLockInterval
			return nil, resource.VersionResult{}, runtime.ProcessResult{}, false, nil
		}

		if !acquired {
			lockLogger.Debug("did-not-get-lock")
			return nil, resource.VersionResult{}, runtime.ProcessResult{}, false, nil
		}

		defer lock.Release()

		volume, versionResult, processResult, err := step.performGetAndInitCache(ctx, logger, delegate, getResource, resourceCache, workerSpec, containerSpec, containerOwner, worker)
		if err != nil {
			return nil, resource.VersionResult{}, runtime.ProcessResult{}, false, err
		}

		return volume, versionResult, processResult, true, nil
	}

	volume, versionResult, processResult, ok, err := attemptGet()
	if err != nil {
		return nil, resource.VersionResult{}, runtime.ProcessResult{}, err
	}
	if ok {
		return volume, versionResult, processResult, nil
	}

	// Resource not cached and failed to acquire lock. Try again after
	// GetResourceLockInterval.
	fmt.Fprintln(delegate.Stderr(), "\x1b[1;36mINFO: waiting to acquire resource lock\x1b[0m")
	fmt.Fprintln(delegate.Stderr(), "")

	ticker := time.NewTicker(GetResourceLockInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, resource.VersionResult{}, runtime.ProcessResult{}, ctx.Err()
		case <-ticker.C:
			volume, versionResult, processResult, ok, err := attemptGet()
			if err != nil {
				return nil, resource.VersionResult{}, runtime.ProcessResult{}, err
			}
			if ok {
				return volume, versionResult, processResult, nil
			}
			// Still can't acquire that darn lock. Wait another interval.
		}
	}
}

func (step *GetStep) retrieveFromCache(
	logger lager.Logger,
	resourceCache db.ResourceCache,
	workerSpec worker.Spec,
	worker runtime.Worker, // may be nil, in which case all compatible workers are possible candidates
) (runtime.Volume, resource.VersionResult, bool, error) {
	var (
		volume runtime.Volume
		found  bool
		err    error
	)
	if worker == nil {
		volume, found, err = step.workerPool.FindResourceCacheVolume(logger, step.metadata.TeamID, resourceCache, workerSpec)
	} else {
		volume, found, err = step.workerPool.FindResourceCacheVolumeOnWorker(logger, resourceCache, workerSpec, worker.Name())
	}
	if err != nil {
		return nil, resource.VersionResult{}, false, err
	}
	if !found {
		return nil, resource.VersionResult{}, false, nil
	}

	metadata, err := step.resourceCacheFactory.ResourceCacheMetadata(resourceCache)
	if err != nil {
		return nil, resource.VersionResult{}, false, err
	}
	result := resource.VersionResult{
		Version:  resourceCache.Version(),
		Metadata: metadata.ToATCMetadata(),
	}
	return volume, result, true, nil
}

// Must be called under a global database lock unique to the resource cache
// signature, or unique to the resource cache and the worker if caching
// streamed volumes is disabled.
func (step *GetStep) performGetAndInitCache(
	ctx context.Context,
	logger lager.Logger,
	delegate GetDelegate,
	getResource resource.Resource,
	resourceCache db.ResourceCache,
	workerSpec worker.Spec,
	containerSpec runtime.ContainerSpec,
	containerOwner db.ContainerOwner,
	worker runtime.Worker, // may be nil, in which case we must select a worker
) (runtime.Volume, resource.VersionResult, runtime.ProcessResult, error) {
	logger = logger.Session("perform-get")
	ctx = lagerctx.NewContext(ctx, logger)

	// We haven't yet selected a worker. This will be the case if
	// EnableCacheStreamedVolumes is true, since we don't need a worker up
	// front.
	if worker == nil {
		var err error
		worker, err = step.workerPool.FindOrSelectWorker(ctx, containerOwner, containerSpec, workerSpec, step.strategy, delegate)
		if err != nil {
			logger.Error("failed-to-select-worker", err)
			return nil, resource.VersionResult{}, runtime.ProcessResult{}, err
		}

		delegate.SelectedWorker(logger, worker.Name())

		defer func() {
			step.workerPool.ReleaseWorker(
				logger,
				containerSpec,
				worker,
				step.strategy,
			)
		}()
	}

	ctx, cancel, err := MaybeTimeout(ctx, step.plan.Timeout)
	if err != nil {
		return nil, resource.VersionResult{}, runtime.ProcessResult{}, err
	}
	ctx = lagerctx.NewContext(ctx, logger)

	defer cancel()

	container, mounts, err := worker.FindOrCreateContainer(ctx, containerOwner, step.containerMetadata, containerSpec)
	if err != nil {
		logger.Error("failed-to-create-container", err)
		return nil, resource.VersionResult{}, runtime.ProcessResult{}, err
	}

	versionResult, processResult, err := getResource.Get(ctx, container, delegate.Stderr())
	if err != nil {
		logger.Error("failed-to-get-resource", err)
		return nil, resource.VersionResult{}, runtime.ProcessResult{}, err
	}

	if processResult.ExitStatus != 0 {
		return nil, versionResult, processResult, nil
	}

	volume := step.resourceMountVolume(mounts)

	if err := volume.InitializeResourceCache(logger, resourceCache); err != nil {
		logger.Error("failed-to-initialize-resource-cache", err)
		return nil, resource.VersionResult{}, runtime.ProcessResult{}, err
	}

	if err := step.resourceCacheFactory.UpdateResourceCacheMetadata(resourceCache, versionResult.Metadata); err != nil {
		logger.Error("failed-to-update-resource-cache-metadata", err)
		return nil, resource.VersionResult{}, runtime.ProcessResult{}, err
	}

	return volume, versionResult, processResult, nil
}

func (step *GetStep) resourceMountVolume(mounts []runtime.VolumeMount) runtime.Volume {
	for _, mnt := range mounts {
		if mnt.MountPath == resource.ResourcesDir("get") {
			return mnt.Volume
		}
	}
	return nil
}
