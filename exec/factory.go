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
	// Get constructs a Get step.
	Get(
		lager.Logger,
		atc.Plan,
		db.Build,
		StepMetadata,
		db.ContainerMetadata,
		GetDelegate,
	) Step

	// Put constructs a Put step.
	Put(
		lager.Logger,
		atc.Plan,
		db.Build,
		StepMetadata,
		db.ContainerMetadata,
		PutDelegate,
	) Step

	// Task constructs a Task step.
	Task(
		lager.Logger,
		atc.Plan,
		db.Build,
		db.ContainerMetadata,
		TaskDelegate,
	) Step
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

	Errored(lager.Logger, string)
}

// Privileged is used to indicate whether the given step should run with
// special privileges (i.e. as an administrator user).
type Privileged bool
