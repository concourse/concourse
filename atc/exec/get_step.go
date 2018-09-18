package exec

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

//go:generate counterfeiter . GetDelegate

type GetDelegate interface {
	BuildStepDelegate

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
		db.NewBuildStepContainerOwner(step.buildID, step.planID),
	)

	versionedSource, err := step.resourceFetcher.Fetch(
		ctx,
		logger,
		resource.Session{
			Metadata: step.containerMetadata,
		},
		step.tags,
		step.teamID,
		step.resourceTypes,
		resourceInstance,
		step.stepMetadata,
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

	state.Artifacts().RegisterSource(worker.ArtifactName(step.name), &getArtifactSource{
		logger:           logger,
		resourceInstance: resourceInstance,
		versionedSource:  versionedSource,
	})

	if step.resource != "" {
		err := step.build.SaveInput(db.BuildInput{
			Name: step.name,
			VersionedResource: db.VersionedResource{
				Resource: step.resource,
				Type:     step.resourceType,
				Version:  db.ResourceVersion(versionedSource.Version()),
				Metadata: db.NewResourceMetadataFields(versionedSource.Metadata()),
			},
		})
		if err != nil {
			logger.Error("failed-to-save-input", err)
			return nil
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
	logger           lager.Logger
	resourceInstance resource.ResourceInstance
	versionedSource  resource.VersionedSource
}

// VolumeOn locates the cache for the GetStep's resource and version on the
// given worker.
func (s *getArtifactSource) VolumeOn(worker worker.Worker) (worker.Volume, bool, error) {
	return s.resourceInstance.FindOn(s.logger.Session("volume-on"), worker)
}

// StreamTo streams the resource's data to the destination.
func (s *getArtifactSource) StreamTo(destination worker.ArtifactDestination) error {
	out, err := s.versionedSource.StreamOut(".")
	if err != nil {
		return err
	}

	defer out.Close()

	return destination.StreamIn(".", out)
}

// StreamFile streams a single file out of the resource.
func (s *getArtifactSource) StreamFile(path string) (io.ReadCloser, error) {
	out, err := s.versionedSource.StreamOut(path)
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
