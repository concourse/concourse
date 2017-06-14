package exec

import (
	"archive/tar"
	"bytes"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

// PutStep produces a resource version using preconfigured params and any data
// available in the worker.ArtifactRepository.
type PutStep struct {
	logger          lager.Logger
	resourceConfig  atc.ResourceConfig
	params          atc.Params
	stepMetadata    StepMetadata
	session         resource.Session
	tags            atc.Tags
	teamID          int
	buildID         int
	planID          atc.PlanID
	delegate        PutDelegate
	resourceFactory resource.ResourceFactory
	resourceTypes   atc.VersionedResourceTypes

	repository *worker.ArtifactRepository

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
	teamID int,
	buildID int,
	planID atc.PlanID,
	delegate PutDelegate,
	resourceFactory resource.ResourceFactory,
	resourceTypes atc.VersionedResourceTypes,
) PutStep {
	return PutStep{
		logger:          logger,
		resourceConfig:  resourceConfig,
		params:          params,
		stepMetadata:    stepMetadata,
		session:         session,
		tags:            tags,
		teamID:          teamID,
		buildID:         buildID,
		planID:          planID,
		delegate:        delegate,
		resourceFactory: resourceFactory,
		resourceTypes:   resourceTypes,
	}
}

// Using finishes construction of the PutStep and returns a *PutStep. If the
// *PutStep errors, its error is reported to the delegate.
func (step PutStep) Using(prev Step, repo *worker.ArtifactRepository) Step {
	step.repository = repo

	return errorReporter{
		Step:          &step,
		ReportFailure: step.delegate.Failed,
	}
}

// Run chooses a worker that supports the step's resource type and creates a
// container.
//
// All worker.ArtifactSources present in the worker.ArtifactRepository are then brought into
// the container, using volumes if possible, and streaming content over if not.
//
// The resource's put script is then invoked. The PutStep is ready as soon as
// the resource's script starts, and signals will be forwarded to the script.
func (step *PutStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	step.delegate.Initializing()

	containerSpec := worker.ContainerSpec{
		ImageSpec: worker.ImageSpec{
			ResourceType: step.resourceConfig.Type,
		},
		Tags:   step.tags,
		TeamID: step.teamID,

		Dir: resource.ResourcesDir("put"),

		Env: step.stepMetadata.Env(),
	}

	for name, source := range step.repository.AsMap() {
		containerSpec.Inputs = append(containerSpec.Inputs, &putInputSource{
			name:   name,
			source: resourceSource{source},
		})
	}

	putResource, err := step.resourceFactory.NewResource(
		step.logger,
		signals,
		db.ForBuild(step.buildID),
		db.NewBuildStepContainerOwner(step.buildID, step.planID),
		step.session.Metadata,
		containerSpec,
		step.resourceTypes,
		step.delegate,
	)
	if err != nil {
		return err
	}

	step.resource = putResource

	step.versionedSource, err = step.resource.Put(
		resource.IOConfig{
			Stdout: step.delegate.Stdout(),
			Stderr: step.delegate.Stderr(),
		},
		step.resourceConfig.Source,
		step.params,
		signals,
		ready,
	)

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
	worker.ArtifactSource
}

func (source resourceSource) StreamTo(dest worker.ArtifactDestination) error {
	return source.ArtifactSource.StreamTo(worker.ArtifactDestination(dest))
}

type emptySource struct {
	worker.ArtifactSource
}

func (source emptySource) StreamTo(dest worker.ArtifactDestination) error {
	emptyTar := new(bytes.Buffer)

	err := tar.NewWriter(emptyTar).Close()
	if err != nil {
		return err
	}

	err = dest.StreamIn(".", emptyTar)
	if err != nil {
		return err
	}

	return nil
}

type putInputSource struct {
	name   worker.ArtifactName
	source worker.ArtifactSource
}

func (s *putInputSource) Name() worker.ArtifactName     { return s.name }
func (s *putInputSource) Source() worker.ArtifactSource { return s.source }

func (s *putInputSource) DestinationPath() string {
	return resource.ResourcesDir("put/" + string(s.name))
}
