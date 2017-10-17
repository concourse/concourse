package exec

import (
	"archive/tar"
	"io"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

// GetAction will fetch a version of a resource on a worker that supports the
// resource type.
type GetAction struct {
	Type          string
	Name          string
	Resource      string
	Source        creds.Source
	Params        creds.Params
	VersionSource VersionSource
	Tags          atc.Tags
	Outputs       []string

	buildStepDelegate      BuildStepDelegate
	resourceFetcher        resource.Fetcher
	teamID                 int
	buildID                int
	planID                 atc.PlanID
	containerMetadata      db.ContainerMetadata
	dbResourceCacheFactory db.ResourceCacheFactory
	stepMetadata           StepMetadata

	resourceTypes creds.VersionedResourceTypes

	versionInfo VersionInfo
	exitStatus  ExitStatus
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
func (action *GetAction) Run(
	logger lager.Logger,
	repository *worker.ArtifactRepository,

	// TODO: consider passing these as context
	signals <-chan os.Signal,
	ready chan<- struct{},
) error {
	version, err := action.VersionSource.GetVersion()
	if err != nil {
		return err
	}

	source, err := action.Source.Evaluate()
	if err != nil {
		return err
	}

	params, err := action.Params.Evaluate()
	if err != nil {
		return err
	}

	resourceCache, err := action.dbResourceCacheFactory.FindOrCreateResourceCache(
		logger,
		db.ForBuild(action.buildID),
		action.Type,
		version,
		source,
		params,
		action.resourceTypes,
	)
	if err != nil {
		logger.Error("failed-to-create-resource-cache", err)
		return err
	}

	resourceInstance := resource.NewResourceInstance(
		resource.ResourceType(action.Type),
		version,
		source,
		params,
		action.resourceTypes,
		resourceCache,
		db.NewBuildStepContainerOwner(action.buildID, action.planID),
	)

	versionedSource, err := action.resourceFetcher.Fetch(
		logger,
		resource.Session{
			Metadata: action.containerMetadata,
		},
		action.Tags,
		action.teamID,
		action.resourceTypes,
		resourceInstance,
		action.stepMetadata,
		action.buildStepDelegate,
		signals,
		ready,
	)
	if err != nil {
		logger.Error("failed-to-fetch-resource", err)
		if err, ok := err.(resource.ErrResourceScriptFailed); ok {
			action.exitStatus = ExitStatus(err.ExitStatus)
			return nil
		}

		return err
	}

	for _, outputName := range action.Outputs {
		repository.RegisterSource(worker.ArtifactName(outputName), &getArtifactSource{
			logger:           logger,
			resourceInstance: resourceInstance,
			versionedSource:  versionedSource,
		})
	}

	action.versionInfo = VersionInfo{
		Version:  versionedSource.Version(),
		Metadata: versionedSource.Metadata(),
	}
	action.exitStatus = ExitStatus(0)

	return nil
}

// VersionInfo returns the fetched or cached resource's version
// and metadata.
func (action *GetAction) VersionInfo() VersionInfo {
	return action.versionInfo
}

// ExitStatus returns exit status of resource get script.
func (action *GetAction) ExitStatus() ExitStatus {
	return action.exitStatus
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

	tarReader := tar.NewReader(out)

	_, err = tarReader.Next()
	if err != nil {
		return nil, FileNotFoundError{Path: path}
	}

	return fileReadCloser{
		Reader: tarReader,
		Closer: out,
	}, nil
}
