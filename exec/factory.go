package exec

import (
	"io"

	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/clock"
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
		worker.Metadata,
		GetDelegate,
		atc.ResourceConfig,
		atc.Tags,
		atc.Params,
		atc.Version,
		atc.ResourceTypes,
	) StepFactory

	// Put constructs a PutStep factory.
	Put(
		lager.Logger,
		StepMetadata,
		worker.Identifier,
		worker.Metadata,
		PutDelegate,
		atc.ResourceConfig,
		atc.Tags,
		atc.Params,
		atc.ResourceTypes,
	) StepFactory

	// DependentGet constructs a GetStep factory whose version is determined by
	// the previous step.
	DependentGet(
		lager.Logger,
		StepMetadata,
		SourceName,
		worker.Identifier,
		worker.Metadata,
		GetDelegate,
		atc.ResourceConfig,
		atc.Tags,
		atc.Params,
		atc.ResourceTypes,
	) StepFactory

	// Task constructs a TaskStep factory.
	Task(
		lager.Logger,
		SourceName,
		worker.Identifier,
		worker.Metadata,
		TaskDelegate,
		Privileged,
		atc.Tags,
		TaskConfigSource,
		atc.ResourceTypes,
		map[string]string,
		map[string]string,
		string,
		clock.Clock,
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

	ImageVersionDetermined(worker.VolumeIdentifier) error

	Stdout() io.Writer
	Stderr() io.Writer
}

// ResourceDelegate is used to record events related to a resource's runtime
// behavior.
type ResourceDelegate interface {
	Initializing()

	Completed(ExitStatus, *VersionInfo)
	Failed(error)

	ImageVersionDetermined(worker.VolumeIdentifier) error

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
