package exec

import (
	"archive/tar"
	"context"
	"fmt"
	"io"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/DataDog/zstd"
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
	build db.Build

	name          string
	resourceType  string
	resource      string
	source        creds.Source
	params        creds.Params
	versionSource VersionSource
	tags          atc.Tags

	delegate GetDelegate

	resourceFetcher        resource.Fetcher
	teamID                 int
	buildID                int
	planID                 atc.PlanID
	containerMetadata      db.ContainerMetadata
	dbResourceCacheFactory db.ResourceCacheFactory
	stepMetadata           StepMetadata

	resourceTypes creds.VersionedResourceTypes

	succeeded bool

	strategy   worker.ContainerPlacementStrategy
	workerPool worker.Pool
}

func NewGetStep(
	build db.Build,

	name string,
	resourceType string,
	resource string,
	source creds.Source,
	params creds.Params,
	versionSource VersionSource,
	tags atc.Tags,

	delegate GetDelegate,

	resourceFetcher resource.Fetcher,
	teamID int,
	buildID int,
	planID atc.PlanID,
	containerMetadata db.ContainerMetadata,
	dbResourceCacheFactory db.ResourceCacheFactory,
	stepMetadata StepMetadata,

	resourceTypes creds.VersionedResourceTypes,

	strategy worker.ContainerPlacementStrategy,
	workerPool worker.Pool,
) Step {
	return &GetStep{
		build: build,

		name:          name,
		resourceType:  resourceType,
		resource:      resource,
		source:        source,
		params:        params,
		versionSource: versionSource,
		tags:          tags,

		delegate: delegate,

		resourceFetcher:        resourceFetcher,
		teamID:                 teamID,
		buildID:                buildID,
		planID:                 planID,
		containerMetadata:      containerMetadata,
		dbResourceCacheFactory: dbResourceCacheFactory,
		stepMetadata:           stepMetadata,

		resourceTypes: resourceTypes,

		strategy:   strategy,
		workerPool: workerPool,
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
		"step-name": step.name,
		"job-id":    step.build.JobID(),
	})

	step.delegate.Initializing(logger)

	version, err := step.versionSource.Version(state)
	if err != nil {
		return err
	}

	source, err := step.source.Evaluate()
	if err != nil {
		return err
	}

	params, err := step.params.Evaluate()
	if err != nil {
		return err
	}

	resourceCache, err := step.dbResourceCacheFactory.FindOrCreateResourceCache(
		logger,
		db.ForBuild(step.buildID),
		step.resourceType,
		version,
		source,
		params,
		step.resourceTypes,
	)
	if err != nil {
		logger.Error("failed-to-create-resource-cache", err)
		return err
	}

	resourceInstance := resource.NewResourceInstance(
		resource.ResourceType(step.resourceType),
		version,
		source,
		params,
		step.resourceTypes,
		resourceCache,
		db.NewBuildStepContainerOwner(step.buildID, step.planID, step.teamID),
	)

	containerSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: step.resourceType,
		},
		TeamID: step.teamID,
		Env:    step.stepMetadata.Env(),
	}

	workerSpec := worker.WorkerSpec{
		ResourceType:  step.resourceType,
		Tags:          step.tags,
		TeamID:        step.teamID,
		ResourceTypes: step.resourceTypes,
	}

	chosenWorker, err := step.workerPool.FindOrChooseWorkerForContainer(logger, resourceInstance.ContainerOwner(), containerSpec, workerSpec, step.strategy)
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
		step.resourceTypes,
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

	state.Artifacts().RegisterSource(artifact.Name(step.name), &getArtifactSource{
		resourceInstance: resourceInstance,
		versionedSource:  versionedSource,
	})

	if step.resource != "" {
		pipeline, found, err := step.build.Pipeline()
		if err != nil {
			logger.Error("failed-to-find-pipeline", err, lager.Data{"name": step.name, "pipeline-name": step.build.PipelineName(), "pipeline-id": step.build.PipelineID()})
			return err
		}

		if !found {
			logger.Debug("pipeline-not-found", lager.Data{"name": step.name, "pipeline-name": step.build.PipelineName(), "pipeline-id": step.build.PipelineID()})
			return ErrPipelineNotFound{step.build.PipelineName()}
		}

		resource, found, err := pipeline.Resource(step.resource)
		if err != nil {
			logger.Error("failed-to-find-resource", err, lager.Data{"name": step.name, "pipeline-name": step.build.PipelineName(), "resource": step.resource})
			return err
		}

		if !found {
			logger.Debug("resource-not-found", lager.Data{"name": step.name, "pipeline-name": step.build.PipelineName(), "resource": step.resource})
			return ErrResourceNotFound{step.resource}
		}

		// Find or Save* the version used in the get step, and update the Metadata
		// *saving will occur when the resource's config has changed, but it hasn't
		// checked yet, so the resource config versions don't exist
		_, err = resource.SaveUncheckedVersion(versionedSource.Version(), db.NewResourceConfigMetadataFields(versionedSource.Metadata()), resourceCache.ResourceConfig(), step.resourceTypes)
		if err != nil {
			logger.Error("failed-to-save-resource-config-version", err, lager.Data{"name": step.name, "resource": step.resource, "version": versionedSource.Version()})
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

	zstdReader := zstd.NewReader(out)
	tarReader := tar.NewReader(zstdReader)

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
