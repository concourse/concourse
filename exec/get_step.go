package exec

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

// GetStep will fetch a version of a resource on a worker that supports the
// resource type.
type GetStep struct {
	logger           lager.Logger
	sourceName       worker.ArtifactName
	resourceConfig   atc.ResourceConfig
	version          atc.Version
	params           atc.Params
	resourceInstance resource.ResourceInstance
	stepMetadata     StepMetadata
	session          resource.Session
	tags             atc.Tags
	teamID           int
	delegate         GetDelegate
	resourceFetcher  resource.Fetcher
	resourceTypes    atc.ResourceTypes

	repository *worker.ArtifactRepository

	fetchSource resource.FetchSource

	succeeded bool

	containerSuccessTTL time.Duration
	containerFailureTTL time.Duration
}

func newGetStep(
	logger lager.Logger,
	sourceName worker.ArtifactName,
	resourceConfig atc.ResourceConfig,
	version atc.Version,
	params atc.Params,
	resourceInstance resource.ResourceInstance,
	stepMetadata StepMetadata,
	session resource.Session,
	tags atc.Tags,
	teamID int,
	delegate GetDelegate,
	resourceFetcher resource.Fetcher,
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
		resourceInstance:    resourceInstance,
		stepMetadata:        stepMetadata,
		session:             session,
		tags:                tags,
		teamID:              teamID,
		delegate:            delegate,
		resourceFetcher:     resourceFetcher,
		resourceTypes:       resourceTypes,
		containerSuccessTTL: containerSuccessTTL,
		containerFailureTTL: containerFailureTTL,
	}
}

// Using finishes construction of the GetStep and returns a *GetStep. If the
// *GetStep errors, its error is reported to the delegate.
func (step GetStep) Using(prev Step, repo *worker.ArtifactRepository) Step {
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

	resourceDefinition := &getStepResource{
		source:       step.resourceConfig.Source,
		resourceType: resource.ResourceType(step.resourceConfig.Type),
		delegate:     step.delegate,
		params:       step.params,
		version:      step.version,
	}

	var err error
	step.fetchSource, err = step.resourceFetcher.Fetch(
		step.logger,
		runSession,
		step.tags,
		step.teamID,
		step.resourceTypes,
		step.resourceInstance,
		step.stepMetadata,
		step.delegate,
		resourceDefinition,
		signals,
		ready,
	)

	if err, ok := err.(resource.ErrResourceScriptFailed); ok {
		step.logger.Error("get-run-resource-script-failed", err)
		step.delegate.Completed(ExitStatus(err.ExitStatus), nil)
		return nil
	}

	if err != nil {
		step.logger.Error("failed-to-init-with-cache", err)
		return err
	}

	step.registerAndReportResource()

	return nil
}

func (step *GetStep) Release() {
	if step.fetchSource == nil {
		return
	}

	if step.succeeded {
		step.fetchSource.Release(worker.FinalTTL(step.containerSuccessTTL))
	} else {
		step.fetchSource.Release(worker.FinalTTL(step.containerFailureTTL))
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
			Version:  step.fetchSource.VersionedSource().Version(),
			Metadata: step.fetchSource.VersionedSource().Metadata(),
		}
		return true

	default:
		return false
	}
}

// VolumeOn locates the cache for the GetStep's resource and version on the
// given worker.
func (step *GetStep) VolumeOn(worker worker.Worker) (worker.Volume, bool, error) {
	return step.resourceInstance.FindOn(step.logger.Session("volume-on"), worker)
}

// StreamTo streams the resource's data to the destination.
func (step *GetStep) StreamTo(destination worker.ArtifactDestination) error {
	out, err := step.fetchSource.VersionedSource().StreamOut(".")
	if err != nil {
		return err
	}

	defer out.Close()

	return destination.StreamIn(".", out)
}

// StreamFile streams a single file out of the resource.
func (step *GetStep) StreamFile(path string) (io.ReadCloser, error) {
	out, err := step.fetchSource.VersionedSource().StreamOut(path)
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

func (step *GetStep) registerAndReportResource() {
	step.repository.RegisterSource(step.sourceName, step)

	step.succeeded = true
	step.delegate.Completed(ExitStatus(0), &VersionInfo{
		Version:  step.fetchSource.VersionedSource().Version(),
		Metadata: step.fetchSource.VersionedSource().Metadata(),
	})
}

type getStepResource struct {
	delegate     GetDelegate
	resourceType resource.ResourceType
	source       atc.Source
	params       atc.Params
	version      atc.Version
}

func (d *getStepResource) IOConfig() resource.IOConfig {
	return resource.IOConfig{
		Stdout: d.delegate.Stdout(),
		Stderr: d.delegate.Stderr(),
	}
}

func (d *getStepResource) Source() atc.Source {
	return d.source
}

func (d *getStepResource) Params() atc.Params {
	return d.params
}

func (d *getStepResource) Version() atc.Version {
	return d.version
}

func (d *getStepResource) ResourceType() resource.ResourceType {
	return d.resourceType
}

func (d *getStepResource) LockName(workerName string) (string, error) {
	id := &getStepLockID{
		Type:       d.resourceType,
		Version:    d.version,
		Source:     d.source,
		Params:     d.params,
		WorkerName: workerName,
	}

	taskNameJSON, err := json.Marshal(id)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(taskNameJSON)), nil
}

type getStepLockID struct {
	Type       resource.ResourceType `json:"type"`
	Version    atc.Version           `json:"version"`
	Source     atc.Source            `json:"source"`
	Params     atc.Params            `json:"params"`
	WorkerName string                `json:"worker_name"`
}
