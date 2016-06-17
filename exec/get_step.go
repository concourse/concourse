package exec

import (
	"archive/tar"
	"fmt"
	"io"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/lager"
)

// GetStep will fetch a version of a resource on a worker that supports the
// resource type.
type GetStep struct {
	logger          lager.Logger
	sourceName      SourceName
	resourceConfig  atc.ResourceConfig
	version         atc.Version
	params          atc.Params
	cacheIdentifier resource.CacheIdentifier
	stepMetadata    StepMetadata
	session         resource.Session
	tags            atc.Tags
	delegate        GetDelegate
	tracker         resource.Tracker
	resourceTypes   atc.ResourceTypes

	repository *SourceRepository

	resource resource.Resource

	versionedSource resource.VersionedSource

	succeeded bool
}

func newGetStep(
	logger lager.Logger,
	sourceName SourceName,
	resourceConfig atc.ResourceConfig,
	version atc.Version,
	params atc.Params,
	cacheIdentifier resource.CacheIdentifier,
	stepMetadata StepMetadata,
	session resource.Session,
	tags atc.Tags,
	delegate GetDelegate,
	tracker resource.Tracker,
	resourceTypes atc.ResourceTypes,
) GetStep {
	return GetStep{
		logger:          logger,
		sourceName:      sourceName,
		resourceConfig:  resourceConfig,
		version:         version,
		params:          params,
		cacheIdentifier: cacheIdentifier,
		stepMetadata:    stepMetadata,
		session:         session,
		tags:            tags,
		delegate:        delegate,
		tracker:         tracker,
		resourceTypes:   resourceTypes,
	}
}

// Using finishes construction of the GetStep and returns a *GetStep. If the
// *GetStep errors, its error is reported to the delegate.
func (step GetStep) Using(prev Step, repo *SourceRepository) Step {
	step.repository = repo

	return errorReporter{
		Step:          &step,
		ReportFailure: step.delegate.Failed,
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
func (step *GetStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	step.delegate.Initializing()

	runSession := step.session
	runSession.ID.Stage = db.ContainerStageRun

	trackedResource, cache, err := step.tracker.InitWithCache(
		step.logger,
		step.stepMetadata,
		runSession,
		resource.ResourceType(step.resourceConfig.Type),
		step.tags,
		step.cacheIdentifier,
		step.resourceTypes,
		step.delegate,
	)
	if err != nil {
		step.logger.Error("failed-to-initialize-resource", err)
		return err
	}

	step.resource = trackedResource

	step.versionedSource = step.resource.Get(
		resource.IOConfig{
			Stdout: step.delegate.Stdout(),
			Stderr: step.delegate.Stderr(),
		},
		step.resourceConfig.Source,
		step.params,
		step.version,
	)

	isInitialized, err := cache.IsInitialized()
	if err != nil {
		step.logger.Error("failed-to-check-if-cache-is-initialized", err)
		return err
	}

	if isInitialized {
		step.logger.Debug("cache-already-initialized")

		fmt.Fprintf(step.delegate.Stdout(), "using version of resource found in cache\n")
		close(ready)
	} else {
		step.logger.Debug("cache-not-initialized")

		err = step.versionedSource.Run(signals, ready)

		if err, ok := err.(resource.ErrResourceScriptFailed); ok {
			step.delegate.Completed(ExitStatus(err.ExitStatus), nil)
			return nil
		}

		if err == resource.ErrAborted {
			return ErrInterrupted
		}

		if err != nil {
			step.logger.Error("failed-to-run-get", err)
			return err
		}

		err = cache.Initialize()
		if err != nil {
			step.logger.Error("failed-to-initialize-cache", err)
		}
	}

	step.repository.RegisterSource(step.sourceName, step)

	step.succeeded = true
	step.delegate.Completed(ExitStatus(0), &VersionInfo{
		Version:  step.versionedSource.Version(),
		Metadata: step.versionedSource.Metadata(),
	})

	return nil
}

// Release releases the resource's container with default infinite TTL(and thus volumes).
// Container reaper checks for successful builds and set the containers' ttl to 5 minutes.
// Container reaper also checks for unsuccessful (failed, aborted, errored) builds
// that are not the latest builds of a job, and release their containers in 5 minutes
func (step *GetStep) Release() {
	if step.resource == nil {
		return
	}

	step.resource.Release(worker.FinalTTL(worker.FinishedContainerTTL))
}

// Result indicates Success as true if the script completed successfully (or
// didn't have to run) and everything else worked fine.
//
// It also indicates VersionInfo with the fetched or cached resource's version
// and metadata.
//
// All other types are ignored.
func (step *GetStep) Result(x interface{}) bool {
	switch v := x.(type) {
	case *Success:
		*v = Success(step.succeeded)
		return true

	case *VersionInfo:
		*v = VersionInfo{
			Version:  step.versionedSource.Version(),
			Metadata: step.versionedSource.Metadata(),
		}
		return true

	default:
		return false
	}
}

// VolumeOn locates the cache for the GetStep's resource and version on the
// given worker.
func (step *GetStep) VolumeOn(worker worker.Worker) (worker.Volume, bool, error) {
	return step.cacheIdentifier.FindOn(step.logger.Session("volume-on"), worker)
}

// StreamTo streams the resource's data to the destination.
func (step *GetStep) StreamTo(destination ArtifactDestination) error {
	out, err := step.versionedSource.StreamOut(".")
	if err != nil {
		return err
	}

	defer out.Close()

	return destination.StreamIn(".", out)
}

// StreamFile streams a single file out of the resource.
func (step *GetStep) StreamFile(path string) (io.ReadCloser, error) {
	out, err := step.versionedSource.StreamOut(path)
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
