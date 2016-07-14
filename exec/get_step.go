package exec

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"time"

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

	containerSuccessTTL time.Duration
	containerFailureTTL time.Duration
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
	containerSuccessTTL time.Duration,
	containerFailureTTL time.Duration,
) GetStep {
	return GetStep{
		logger:              logger,
		sourceName:          sourceName,
		resourceConfig:      resourceConfig,
		version:             version,
		params:              params,
		cacheIdentifier:     cacheIdentifier,
		stepMetadata:        stepMetadata,
		session:             session,
		tags:                tags,
		delegate:            delegate,
		tracker:             tracker,
		resourceTypes:       resourceTypes,
		containerSuccessTTL: containerSuccessTTL,
		containerFailureTTL: containerFailureTTL,
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

func (step *GetStep) createResourceContainerAndRun(
	chosenWorker worker.Worker,
	cachedVolume worker.Volume,
	runSession resource.Session,
	signals <-chan os.Signal,
	ready chan<- struct{},
) error {
	containerSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: string(resource.ResourceType(step.resourceConfig.Type)),
			Privileged:   true,
		},
		Ephemeral: runSession.Ephemeral,
		Tags:      step.tags,
		Env:       step.stepMetadata.Env(),
		Outputs: []worker.VolumeMount{
			{
				Volume:    cachedVolume,
				MountPath: resource.ResourcesDir("get"),
			},
		},
	}

	step.logger.Debug("createResourceContainerAndRun", lager.Data{"container-id": runSession.ID})
	container, err := chosenWorker.CreateContainer(
		step.logger,
		signals,
		step.delegate,
		runSession.ID,
		runSession.Metadata,
		containerSpec,
		step.resourceTypes,
	)

	if err != nil {
		step.logger.Error("failed-to-create-container", err)
		return err
	}

	step.logger.Info("created-container-in-get", lager.Data{"container": container.Handle()})

	step.logger.Debug("going-to-run-get")
	step.resource.SetContainer(container)
	step.versionedSource = step.resource.Get(
		cachedVolume,
		resource.IOConfig{
			Stdout: step.delegate.Stdout(),
			Stderr: step.delegate.Stderr(),
		},
		step.resourceConfig.Source,
		step.params,
		step.version,
	)

	step.logger.Info("get-step-running")
	err = step.versionedSource.Run(signals, ready)
	step.logger.Info("get-step-run")

	if err, ok := err.(resource.ErrResourceScriptFailed); ok {
		step.logger.Error("get-run-resource-script-failed", err, lager.Data{"container": container.Handle()})
		step.delegate.Completed(ExitStatus(err.ExitStatus), nil)
		return err
	}

	if err == resource.ErrAborted {
		step.logger.Error("get-run-resource-aborted", err, lager.Data{"container": container.Handle()})
		return ErrInterrupted
	}

	if err != nil {
		step.logger.Error("failed-to-run-get", err)
		return err
	}

	return nil
}

func (step *GetStep) registerAndReportResource() {
	step.repository.RegisterSource(step.sourceName, step)

	step.succeeded = true
	step.delegate.Completed(ExitStatus(0), &VersionInfo{
		Version:  step.versionedSource.Version(),
		Metadata: step.versionedSource.Metadata(),
	})
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

	getResource, cache, chosenWorker, foundContainer, err := step.tracker.InitResourceWithCache(
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
		step.logger.Error("failed-to-find-container-for-get-step", err)
		return err
	}

	step.resource = getResource

	isInitialized, err := cache.IsInitialized()
	if err != nil {
		step.logger.Error("failed-to-check-if-cache-is-initialized", err)
		return err
	}

	step.versionedSource = step.resource.Get(
		cache.Volume(),
		resource.IOConfig{
			Stdout: step.delegate.Stdout(),
			Stderr: step.delegate.Stderr(),
		},
		step.resourceConfig.Source,
		step.params,
		step.version,
	)

	if isInitialized {
		step.logger.Debug("cache-already-initialized")

		fmt.Fprintf(step.delegate.Stdout(), "using version of resource found in cache\n")
		close(ready)
	} else {
		step.logger.Debug("cache-not-initialized")
		if !foundContainer {
			step.logger.Debug("cached-volume", lager.Data{"handle": cache.Volume().Handle()})
			err := step.createResourceContainerAndRun(chosenWorker, cache.Volume(), runSession, signals, ready)

			if _, ok := err.(resource.ErrResourceScriptFailed); ok {
				return nil
			}

			if err != nil {
				step.logger.Error("failed-to-create-and-run-get-container", err)
				return err
			}
		}

		err = cache.Initialize()
		if err != nil {
			step.logger.Error("failed-to-initialize-cache", err)
		}
	}

	step.registerAndReportResource()
	return nil
}

func (step *GetStep) Release() {
	if step.resource == nil {
		return
	}

	if step.succeeded {
		step.resource.Release(worker.FinalTTL(step.containerSuccessTTL))
	} else {
		step.resource.Release(worker.FinalTTL(step.containerFailureTTL))
	}
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
