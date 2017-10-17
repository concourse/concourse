package exec

import (
	"io"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

//go:generate counterfeiter . Factory

// Factory is used when building up the steps for a build.
type Factory interface {
	// Get constructs a ActionsStep factory for Get.
	Get(
		lager.Logger,
		atc.Plan,
		db.Build,
		StepMetadata,
		db.ContainerMetadata,
		ActionsBuildEventsDelegate,
		BuildStepDelegate,
	) StepFactory

	// Put constructs a ActionsStep factory for Put.
	Put(
		lager.Logger,
		atc.Plan,
		db.Build,
		StepMetadata,
		db.ContainerMetadata,
		ActionsBuildEventsDelegate,
		BuildStepDelegate,
	) StepFactory

	// Task constructs a ActionsStep factory for Task.
	Task(
		lager.Logger,
		atc.Plan,
		db.Build,
		db.ContainerMetadata,
		TaskBuildEventsDelegate,
		ActionsBuildEventsDelegate,
		BuildStepDelegate,
	) StepFactory
}

// StepMetadata is used to inject metadata to make available to the step when
// it's running.
type StepMetadata interface {
	Env() []string
}

//go:generate counterfeiter . BuildStepDelegate

type BuildStepDelegate interface {
	ImageVersionDetermined(*db.UsedResourceCache) error
	Stdout() io.Writer
	Stderr() io.Writer
}

// Privileged is used to indicate whether the given step should run with
// special privileges (i.e. as an administrator user).
type Privileged bool
