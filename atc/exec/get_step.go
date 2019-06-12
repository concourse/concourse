package exec

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec/artifact"
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
	BuildStepDelegate

	Initializing(lager.Logger)
	Starting(lager.Logger)
	Finished(lager.Logger, ExitStatus, VersionInfo)
}

// GetStep will fetch a version of a resource on a worker that supports the
// resource type.
type GetStep struct {
	planID               atc.PlanID
	plan                 atc.GetPlan
	build                db.Build
	stepMetadata         StepMetadata
	containerMetadata    db.ContainerMetadata
	secrets              creds.Secrets
	resourceFetcher      resource.Fetcher
	resourceCacheFactory db.ResourceCacheFactory
	strategy             worker.ContainerPlacementStrategy
	workerPool           worker.Pool
	delegate             GetDelegate
	succeeded            bool
}

func NewGetStep(
	planID atc.PlanID,
	plan atc.GetPlan,
	build db.Build,
	stepMetadata StepMetadata,
	containerMetadata db.ContainerMetadata,
	secrets creds.Secrets,
	resourceFetcher resource.Fetcher,
	resourceCacheFactory db.ResourceCacheFactory,
	strategy worker.ContainerPlacementStrategy,
	workerPool worker.Pool,
	delegate GetDelegate,
) Step {
	return &GetStep{
		planID:               planID,
		plan:                 plan,
		build:                build,
		stepMetadata:         stepMetadata,
		containerMetadata:    containerMetadata,
		secrets:              secrets,
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
		"job-id":    step.build.JobID(),
	})

	step.delegate.Initializing(logger)

	variables := creds.NewVariables(step.secrets, step.build.TeamName(), step.build.PipelineName())

	source, err := creds.NewSource(variables, step.plan.Source).Evaluate()
	if err != nil {
		return err
	}

	params, err := creds.NewParams(variables, step.plan.Params).Evaluate()
	if err != nil {
		return err
	}

	resourceTypes := creds.NewVersionedResourceTypes(variables, step.plan.VersionedResourceTypes)

	version, err := NewVersionSourceFromPlan(&step.plan).Version(state)
	if err != nil {
		return err
	}

	containerSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: step.plan.Type,
		},
		TeamID: step.build.TeamID(),
		Env:    step.stepMetadata.Env(),
	}

	workerSpec := worker.WorkerSpec{
		ResourceType:  step.plan.Type,
		Tags:          step.plan.Tags,
		TeamID:        step.build.TeamID(),
		ResourceTypes: resourceTypes,
	}

	resourceCache, err := step.resourceCacheFactory.FindOrCreateResourceCache(
		logger,
		db.ForBuild(step.build.ID()),
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
		db.NewBuildStepContainerOwner(step.build.ID(), step.planID, step.build.TeamID()),
	)

	chosenWorker, err := step.workerPool.FindOrChooseWorkerForContainer(
		ctx,
		logger,
		resourceInstance.ContainerOwner(),
		containerSpec,
		step.containerMetadata,
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
		resource.Session{
			Metadata: step.containerMetadata,
		},
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

	if step.plan.Resource != "" {
		pipeline, found, err := step.build.Pipeline()
		if err != nil {
			logger.Error("failed-to-find-pipeline", err, lager.Data{"name": step.plan.Name, "pipeline-name": step.build.PipelineName(), "pipeline-id": step.build.PipelineID()})
			return err
		}

		if !found {
			logger.Debug("pipeline-not-found", lager.Data{"name": step.plan.Name, "pipeline-name": step.build.PipelineName(), "pipeline-id": step.build.PipelineID()})
			return ErrPipelineNotFound{step.build.PipelineName()}
		}

		resource, found, err := pipeline.Resource(step.plan.Resource)
		if err != nil {
			logger.Error("failed-to-find-resource", err, lager.Data{"name": step.plan.Name, "pipeline-name": step.build.PipelineName(), "resource": step.plan.Resource})
			return err
		}

		if !found {
			logger.Debug("resource-not-found", lager.Data{"name": step.plan.Name, "pipeline-name": step.build.PipelineName(), "resource": step.plan.Resource})
			return ErrResourceNotFound{step.plan.Resource}
		}

		// Find or Save* the version used in the get step, and update the Metadata
		// *saving will occur when the resource's config has changed, but it hasn't
		// checked yet, so the resource config versions don't exist
		_, err = resource.SaveUncheckedVersion(versionedSource.Version(), db.NewResourceConfigMetadataFields(versionedSource.Metadata()), resourceCache.ResourceConfig(), resourceTypes)
		if err != nil {
			logger.Error("failed-to-save-resource-config-version", err, lager.Data{"name": step.plan.Name, "resource": step.plan.Resource, "version": versionedSource.Version()})
			return err
		}
	}

	step.succeeded = true

	step.delegate.Finished(logger, 0, VersionInfo{
		Version:  versionedSource.Version(),
		Metadata: versionedSource.Metadata(),
	})

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
func (s *getArtifactSource) StreamTo(logger lager.Logger, destination worker.ArtifactDestination) error {
	return streamToHelper(s.versionedSource, logger, destination)
}

// StreamFile streams a single file out of the resource.
func (s *getArtifactSource) StreamFile(logger lager.Logger, path string) (io.ReadCloser, error) {
	return streamFileHelper(s.versionedSource, logger, path)
}

func streamToHelper(s interface {
	StreamOut(string) (io.ReadCloser, error)
}, logger lager.Logger, destination worker.ArtifactDestination) error {
	logger.Debug("start")

	defer logger.Debug("end")

	out, err := s.StreamOut(".")
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	defer out.Close()

	err = destination.StreamIn(".", out)
	if err != nil {
		logger.Error("failed", err)
		return err
	}
	return nil
}

func streamFileHelper(s interface {
	StreamOut(string) (io.ReadCloser, error)
}, logger lager.Logger, path string) (io.ReadCloser, error) {
	out, err := s.StreamOut(path)
	if err != nil {
		return nil, err
	}

	gzReader, err := gzip.NewReader(out)
	if err != nil {
		return nil, FileNotFoundError{Path: path}
	}

	tarReader := tar.NewReader(gzReader)

	_, err = tarReader.Next()
	if err != nil {
		return nil, FileNotFoundError{Path: path}
	}

	return fileReadCloser{
		Reader: tarReader,
		Closer: out,
	}, nil
}

type fileReadCloser struct {
	io.Reader
	io.Closer
}
