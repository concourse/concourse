package exec

import (
	"archive/tar"
	"fmt"
	"io"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
	"github.com/concourse/baggageclaim"
	"github.com/pivotal-golang/lager"
)

type getStep struct {
	logger          lager.Logger
	sourceName      SourceName
	resourceConfig  atc.ResourceConfig
	version         atc.Version
	params          atc.Params
	cacheIdentifier resource.CacheIdentifier
	stepMetadata    StepMetadata
	session         resource.Session
	tags            atc.Tags
	delegate        ResourceDelegate
	tracker         resource.Tracker

	repository *SourceRepository

	resource resource.Resource

	versionedSource resource.VersionedSource

	exitStatus int
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
	delegate ResourceDelegate,
	tracker resource.Tracker,
) getStep {
	return getStep{
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
	}
}

func (step getStep) Using(prev Step, repo *SourceRepository) Step {
	step.repository = repo

	return failureReporter{
		Step:          &step,
		ReportFailure: step.delegate.Failed,
	}
}

func (step *getStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	trackedResource, cache, err := step.tracker.InitWithCache(
		step.logger,
		step.stepMetadata,
		step.session,
		resource.ResourceType(step.resourceConfig.Type),
		step.tags,
		step.cacheIdentifier,
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
			step.exitStatus = err.ExitStatus
			step.delegate.Completed(ExitStatus(err.ExitStatus), nil)
			return nil
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

	step.exitStatus = 0
	step.delegate.Completed(ExitStatus(0), &VersionInfo{
		Version:  step.versionedSource.Version(),
		Metadata: step.versionedSource.Metadata(),
	})

	return nil
}

func (step *getStep) Release() {
	if step.resource == nil {
		return
	}

	if step.exitStatus == 0 {
		step.resource.Release(0)
	} else {
		step.resource.Release(failedStepTTL)
	}
}

func (step *getStep) Result(x interface{}) bool {
	switch v := x.(type) {
	case *Success:
		*v = step.exitStatus == 0
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

func (step *getStep) VolumeOn(worker worker.Worker) (baggageclaim.Volume, bool, error) {
	vm, hasVM := worker.VolumeManager()
	if !hasVM {
		return nil, false, nil
	}

	return step.cacheIdentifier.FindOn(step.logger.Session("volume-on"), vm)
}

func (step *getStep) StreamTo(destination ArtifactDestination) error {
	out, err := step.versionedSource.StreamOut(".")
	if err != nil {
		return err
	}

	return destination.StreamIn(".", out)
}

func (step *getStep) StreamFile(path string) (io.ReadCloser, error) {
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
