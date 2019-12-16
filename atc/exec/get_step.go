package exec

import (
	"archive/tar"
	"context"
	"fmt"
	"github.com/concourse/concourse/vars"
	"io"

	"github.com/hashicorp/go-multierror"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/DataDog/zstd"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec/artifact"
	"github.com/concourse/concourse/atc/fetcher"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/worker"
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
	Finished(lager.Logger, ExitStatus, VersionInfo)
	Errored(lager.Logger, string)

	UpdateVersion(lager.Logger, atc.GetPlan, VersionInfo)
}

// GetStep will fetch a version of a resource on a worker that supports the
// resource type.
type GetStep struct {
	planID               atc.PlanID
	plan                 atc.GetPlan
	metadata             StepMetadata
	containerMetadata    db.ContainerMetadata
	resourceFetcher      fetcher.Fetcher
	resourceCacheFactory db.ResourceCacheFactory
	strategy             worker.ContainerPlacementStrategy
	workerPool           worker.Pool
	delegate             GetDelegate
	succeeded            bool
}

func NewGetStep(
	planID atc.PlanID,
	plan atc.GetPlan,
	metadata StepMetadata,
	containerMetadata db.ContainerMetadata,
	resourceFetcher fetcher.Fetcher,
	resourceCacheFactory db.ResourceCacheFactory,
	strategy worker.ContainerPlacementStrategy,
	workerPool worker.Pool,
	delegate GetDelegate,
) Step {
	return &GetStep{
		planID:               planID,
		plan:                 plan,
		metadata:             metadata,
		containerMetadata:    containerMetadata,
		resourceFetcher:      resourceFetcher,
		resourceCacheFactory: resourceCacheFactory,
		strategy:             strategy,
		workerPool:           workerPool,
		delegate:             delegate,
	}
}

// Run ultimately registers the configured resource version's ArtifactSource
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
// fetched ArtifactSource is initialized, thus warming the worker's cache.
//
// At the end, the resulting ArtifactSource (either from using the cache or
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

	resourceInstance := resource.NewResourceInstance(
		resource.ResourceType(step.plan.Type),
		version,
		source,
		params,
		resourceTypes,
		resourceCache,
		db.NewBuildStepContainerOwner(step.metadata.BuildID, step.planID, step.metadata.TeamID),
	)

	chosenWorker, err := step.workerPool.FindOrChooseWorkerForContainer(
		ctx,
		logger,
		resourceInstance.ContainerOwner(),
		containerSpec,
		workerSpec,
		step.strategy,
	)
	if err != nil {
		return err
	}

	step.delegate.Starting(logger)

	versionedSource, err := step.resourceFetcher.Fetch(
		ctx,
		logger,
		step.containerMetadata,
		chosenWorker,
		containerSpec,
		resourceTypes,
		resourceInstance,
		step.delegate,
	)
	if err != nil {
		logger.Error("failed-to-fetch-resource", err)

		if err, ok := err.(resource.ErrResourceScriptFailed); ok {
			step.delegate.Finished(logger, ExitStatus(err.ExitStatus), VersionInfo{})
			return nil
		}

		return err
	}

	state.Artifacts().RegisterSource(artifact.Name(step.plan.Name), &getArtifactSource{
		resourceInstance: resourceInstance,
		versionedSource:  versionedSource,
	})

	versionInfo := VersionInfo{
		Version:  versionedSource.Version(),
		Metadata: versionedSource.Metadata(),
	}

	if step.plan.Resource != "" {
		step.delegate.UpdateVersion(logger, step.plan, versionInfo)
	}

	step.succeeded = true

	step.delegate.Finished(logger, 0, versionInfo)

	return nil
}

// Succeeded returns true if the resource was successfully fetched.
func (step *GetStep) Succeeded() bool {
	return step.succeeded
}

type getArtifactSource struct {
	resourceInstance resource.ResourceInstance
	versionedSource  resource.VersionedSource
}

// VolumeOn locates the cache for the GetStep's resource and version on the
// given worker.
func (s *getArtifactSource) VolumeOn(logger lager.Logger, worker worker.Worker) (worker.Volume, bool, error) {
	return s.resourceInstance.FindOn(logger.Session("volume-on"), worker)
}

// StreamTo streams the resource's data to the destination.
func (s *getArtifactSource) StreamTo(ctx context.Context, logger lager.Logger, destination worker.ArtifactDestination) error {
	return streamToHelper(ctx, s.versionedSource, logger, destination)
}

// StreamFile streams a single file out of the resource.
func (s *getArtifactSource) StreamFile(ctx context.Context, logger lager.Logger, path string) (io.ReadCloser, error) {
	return streamFileHelper(ctx, s.versionedSource, logger, path)
}

func streamToHelper(
	ctx context.Context,
	s interface {
		StreamOut(context.Context, string) (io.ReadCloser, error)
	},
	logger lager.Logger,
	destination worker.ArtifactDestination,
) error {
	logger.Debug("start")

	defer logger.Debug("end")

	out, err := s.StreamOut(ctx, ".")
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	defer out.Close()

	err = destination.StreamIn(ctx, ".", out)
	if err != nil {
		logger.Error("failed", err)
		return err
	}
	return nil
}

func streamFileHelper(
	ctx context.Context,
	s interface {
		StreamOut(context.Context, string) (io.ReadCloser, error)
	},
	logger lager.Logger,
	path string,
) (io.ReadCloser, error) {
	out, err := s.StreamOut(ctx, path)
	if err != nil {
		return nil, err
	}

	zstdReader := zstd.NewReader(out)
	tarReader := tar.NewReader(zstdReader)

	_, err = tarReader.Next()
	if err != nil {
		return nil, FileNotFoundError{Path: path}
	}

	return fileReadMultiCloser{
		reader: tarReader,
		closers: []io.Closer{
			out,
			zstdReader,
		},
	}, nil
}

type fileReadMultiCloser struct {
	reader  io.Reader
	closers []io.Closer
}

func (frc fileReadMultiCloser) Read(p []byte) (n int, err error) {
	return frc.reader.Read(p)
}

func (frc fileReadMultiCloser) Close() error {
	var closeErrors error

	for _, closer := range frc.closers {
		err := closer.Close()
		if err != nil {
			closeErrors = multierror.Append(closeErrors, err)
		}
	}

	return closeErrors
}
