package exec

import (
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/atc/resource"
	"github.com/pivotal-golang/lager"
)

// PutStep produces a resource version using preconfigured params and any data
// available in the SourceRepository.
type PutStep struct {
	logger         lager.Logger
	resourceConfig atc.ResourceConfig
	params         atc.Params
	stepMetadata   StepMetadata
	session        resource.Session
	tags           atc.Tags
	delegate       PutDelegate
	tracker        resource.Tracker

	repository *SourceRepository

	resource resource.Resource

	versionedSource resource.VersionedSource

	succeeded bool
}

func newPutStep(
	logger lager.Logger,
	resourceConfig atc.ResourceConfig,
	params atc.Params,
	stepMetadata StepMetadata,
	session resource.Session,
	tags atc.Tags,
	delegate PutDelegate,
	tracker resource.Tracker,
) PutStep {
	return PutStep{
		logger:         logger,
		resourceConfig: resourceConfig,
		params:         params,
		stepMetadata:   stepMetadata,
		session:        session,
		tags:           tags,
		delegate:       delegate,
		tracker:        tracker,
	}
}

// Using finishes construction of the PutStep and returns a *PutStep. If the
// *PutStep errors, its error is reported to the delegate.
func (step PutStep) Using(prev Step, repo *SourceRepository) Step {
	step.repository = repo

	return errorReporter{
		Step:          &step,
		ReportFailure: step.delegate.Failed,
	}
}

// Run chooses a worker that supports the step's resource type and creates a
// container.
//
// All ArtifactSources present in the SourceRepository are then brought into
// the container, using volumes if possible, and streaming content over if not.
//
// The resource's put script is then invoked. The PutStep is ready as soon as
// the resource's script starts, and signals will be forwarded to the script.
func (step *PutStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	step.delegate.Initializing()

	sources := step.repository.AsMap()

	resourceSources := make(map[string]resource.ArtifactSource)
	for name, source := range sources {
		resourceSources[string(name)] = resourceSource{source}
	}

	trackedResource, missingNames, err := step.tracker.InitWithSources(
		step.logger,
		step.stepMetadata,
		step.session,
		resource.ResourceType(step.resourceConfig.Type),
		step.tags,
		resourceSources,
	)

	if err != nil {
		return err
	}

	missingSourceNames := make([]SourceName, len(missingNames))
	for i, n := range missingNames {
		missingSourceNames[i] = SourceName(n)
	}

	step.resource = trackedResource

	scopedRepo, err := step.repository.ScopedTo(missingSourceNames...)
	if err != nil {
		return err
	}

	step.versionedSource = step.resource.Put(
		resource.IOConfig{
			Stdout: step.delegate.Stdout(),
			Stderr: step.delegate.Stderr(),
		},
		step.resourceConfig.Source,
		step.params,
		resourceSource{scopedRepo},
	)

	err = step.versionedSource.Run(signals, ready)

	if err, ok := err.(resource.ErrResourceScriptFailed); ok {
		step.delegate.Completed(ExitStatus(err.ExitStatus), nil)
		return nil
	}

	if err == resource.ErrAborted {
		return ErrInterrupted
	}

	if err != nil {
		return err
	}

	step.succeeded = true
	step.delegate.Completed(ExitStatus(0), &VersionInfo{
		Version:  step.versionedSource.Version(),
		Metadata: step.versionedSource.Metadata(),
	})

	return nil
}

// Release releases the created container for either SuccessfulStepTTL or
// FailedStepTTL.
func (step *PutStep) Release() {
	if step.resource == nil {
		return
	}

	if step.succeeded {
		step.resource.Release(SuccessfulStepTTL)
	} else {
		step.resource.Release(FailedStepTTL)
	}
}

// Result indicates Success as true if the script completed with exit status 0.
//
// It also indicates VersionInfo returned by the script.
//
// Any other type is ignored.
func (step *PutStep) Result(x interface{}) bool {
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

type resourceSource struct {
	ArtifactSource
}

func (source resourceSource) StreamTo(dest resource.ArtifactDestination) error {
	return source.ArtifactSource.StreamTo(resource.ArtifactDestination(dest))
}
