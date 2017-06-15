package exec

import (
	"errors"

	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
	"github.com/tedsuo/ifrit"
)

// ErrInterrupted is returned by steps when they exited as a result of
// receiving a signal.
var ErrInterrupted = errors.New("interrupted")

//go:generate counterfeiter . StepFactory

// StepFactory constructs a step. The previous step and source repository are
// provided.
//
// Some steps, i.e. DependentGetStep, use information from the previous step to
// determine how to run.
// TODO: Remove Step in prev
type StepFactory interface {
	Using(*worker.ArtifactRepository) Step
}

//go:generate counterfeiter . Step

// A Step is an object that can be executed, whose result (e.g. Success) can be
// collected, and whose dependent resources (e.g. Containers, Volumes) can be
// released, allowing them to expire.
type Step interface {
	// Run is called when it's time to execute the step. It should indicate when
	// it's ready, and listen for signals at points where the potential time is
	// unbounded (i.e. running a task or a resource action).
	//
	// Steps wrapping other steps should be careful to propagate signals and
	// indicate that they're ready as soon as their wrapped steps are ready.
	//
	// Steps should return ErrInterrupted if they received a signal that caused
	// them to stop.
	//
	// Steps must be idempotent. Each step is responsible for handling its own
	// idempotency; usually this is done by saving off "checkpoints" in some way
	// that can be checked again if the step starts running again from the start.
	// For example, by having the ID for a container be deterministic and unique
	// for each step, and checking for properties on the container to determine
	// how far the step got.
	ifrit.Runner

	// Succeeded is true when the Step succeeded, and false otherwise.
	// Succeeded is not guaranteed to be truthful until after you run Run()
	Succeeded() bool
}

// Success indicates whether a step completed successfully.
type Success bool

// ExitStatus is the resulting exit code from the process that the step ran.
// Typically if the ExitStatus result is 0, the Success result is true.
type ExitStatus int

// VersionInfo is the version and metadata of a resource that was fetched or
// produced. It is used by Put and Get.
type VersionInfo struct {
	Version  atc.Version
	Metadata []atc.MetadataField
}
