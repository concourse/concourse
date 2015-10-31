package exec

import (
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/atc/resource"
	"github.com/pivotal-golang/lager"
)

type putStep struct {
	logger         lager.Logger
	resourceConfig atc.ResourceConfig
	params         atc.Params
	stepMetadata   StepMetadata
	session        resource.Session
	tags           atc.Tags
	delegate       ResourceDelegate
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
	delegate ResourceDelegate,
	tracker resource.Tracker,
) putStep {
	return putStep{
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

func (step putStep) Using(prev Step, repo *SourceRepository) Step {
	step.repository = repo

	return failureReporter{
		Step:          &step,
		ReportFailure: step.delegate.Failed,
	}
}

func (step *putStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
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

func (step *putStep) Release() {
	if step.resource == nil {
		return
	}

	if step.succeeded {
		step.resource.Release(SuccessfulStepTTL)
	} else {
		step.resource.Release(FailedStepTTL)
	}
}

func (step *putStep) Result(x interface{}) bool {
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
