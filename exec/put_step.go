package exec

import (
	"archive/tar"
	"io"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
	"github.com/concourse/baggageclaim"
	"github.com/pivotal-golang/lager"
)

type putStep struct {
	logger         lager.Logger
	workerPool     worker.Client
	resourceConfig atc.ResourceConfig
	params         atc.Params
	stepMetadata   StepMetadata
	session        resource.Session
	tags           atc.Tags
	delegate       ResourceDelegate
	trackerFactory TrackerFactory

	repository *SourceRepository

	resource resource.Resource

	versionedSource resource.VersionedSource

	exitStatus int
}

func newPutStep(
	logger lager.Logger,
	workerPool worker.Client,
	resourceConfig atc.ResourceConfig,
	params atc.Params,
	stepMetadata StepMetadata,
	session resource.Session,
	tags atc.Tags,
	delegate ResourceDelegate,
	trackerFactory TrackerFactory,
) putStep {
	return putStep{
		logger:         logger,
		workerPool:     workerPool,
		resourceConfig: resourceConfig,
		params:         params,
		stepMetadata:   stepMetadata,
		session:        session,
		tags:           tags,
		delegate:       delegate,
		trackerFactory: trackerFactory,
	}
}

func (step putStep) Using(prev Step, repo *SourceRepository) Step {
	step.repository = repo

	return failureReporter{
		Step:          &step,
		ReportFailure: step.delegate.Failed,
	}
}

func (step *putStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	resourceSpec := worker.WorkerSpec{
		ResourceType: step.resourceConfig.Type,
		Tags:         step.tags,
	}

	chosenWorker, err := step.workerPool.Satisfying(resourceSpec)
	if err != nil {
		return err
	}

	tracker := step.trackerFactory.TrackerFor(chosenWorker)

	trackedResource, err := tracker.Init(
		step.stepMetadata,
		step.session,
		resource.ResourceType(step.resourceConfig.Type),
		step.tags,
		resource.VolumeMount{},
	)
	if err != nil {
		return err
	}

	step.resource = trackedResource

	step.versionedSource = step.resource.Put(
		resource.IOConfig{
			Stdout: step.delegate.Stdout(),
			Stderr: step.delegate.Stderr(),
		},
		step.resourceConfig.Source,
		step.params,
		resourceSource{step.repository},
	)

	err = step.versionedSource.Run(signals, ready)

	if err, ok := err.(resource.ErrResourceScriptFailed); ok {
		step.exitStatus = err.ExitStatus
		step.delegate.Completed(ExitStatus(err.ExitStatus), nil)
		return nil
	}

	if err != nil {
		return err
	}

	step.exitStatus = 0
	step.delegate.Completed(ExitStatus(0), &VersionInfo{
		Version:  step.versionedSource.Version(),
		Metadata: step.versionedSource.Metadata(),
	})

	return nil
}

func (step *putStep) Release() {
	if step.resource != nil {
		step.resource.Release()
	}
}

func (step *putStep) Result(x interface{}) bool {
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

func (step *putStep) VolumeOn(worker.Worker) (baggageclaim.Volume, bool, error) {
	return nil, false, nil
}

func (step *putStep) StreamTo(destination ArtifactDestination) error {
	out, err := step.versionedSource.StreamOut(".")
	if err != nil {
		return err
	}

	return destination.StreamIn(".", out)
}

func (step *putStep) StreamFile(path string) (io.ReadCloser, error) {
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

type resourceSource struct {
	ArtifactSource
}

func (source resourceSource) StreamTo(dest resource.ArtifactDestination) error {
	return source.ArtifactSource.StreamTo(resource.ArtifactDestination(dest))
}
