package exec

import (
	"io"

	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Factory

// Factory is used when building up the steps for a build.
type Factory interface {
	// Get constructs a GetStep factory.
	Get(
		lager.Logger,
		StepMetadata,
		SourceName,
		worker.Identifier,
		GetDelegate,
		atc.ResourceConfig,
		atc.Params,
		atc.Tags,
		atc.Version,
	) StepFactory

	// Put constructs a PutStep factory.
	Put(
		lager.Logger,
		StepMetadata,
		worker.Identifier,
		PutDelegate,
		atc.ResourceConfig,
		atc.Tags,
		atc.Params,
	) StepFactory

	// DependentGet constructs a GetStep factory whose version is determined by
	// the previous step.
	DependentGet(
		lager.Logger,
		StepMetadata,
		SourceName,
		worker.Identifier,
		GetDelegate,
		atc.ResourceConfig,
		atc.Tags,
		atc.Params,
	) StepFactory

	// Task constructs a TaskStep factory.
	Task(
		lager.Logger,
		SourceName,
		worker.Identifier,
		TaskDelegate,
		Privileged,
		atc.Tags,
		TaskConfigSource,
	) StepFactory
}

// StepMetadata is used to inject metadata to make available to the step when
// it's running.
type StepMetadata interface {
	Env() []string
}

//go:generate counterfeiter . TaskDelegate

// TaskDelegate is used to record events related to a TaskStep's runtime
// behavior.
type TaskDelegate interface {
	Initializing(atc.TaskConfig)
	Started()

	Finished(ExitStatus)
	Failed(error)

	Stdout() io.Writer
	Stderr() io.Writer
}

// ResourceDelegate is used to record events related to a resource's runtime
// behavior.
type ResourceDelegate interface {
	Completed(ExitStatus, *VersionInfo)
	Failed(error)

	Stdout() io.Writer
	Stderr() io.Writer
}

//go:generate counterfeiter . GetDelegate

// GetDelegate is used to record events related to a GetStep's runtime
// behavior.
type GetDelegate interface {
	ResourceDelegate
}

//go:generate counterfeiter . PutDelegate

// PutDelegate is used to record events related to a PutStep's runtime
// behavior.
type PutDelegate interface {
	ResourceDelegate
}

// Privileged is used to indicate whether the given step should run with
// special privileges (i.e. as an administrator user).
type Privileged bool
