package exec

import (
	"errors"
	"os"

	"github.com/concourse/atc"
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
type StepFactory interface {
	Using(Step, *SourceRepository) Step
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

	// Release is called when the build has completed and no more steps will be
	// executed in the build plan. Steps with containers should release their
	// containers, with a final TTL of either the configured containerSuccessTTL
	// or containerFailureTTL. The TTL of containers associated with failed builds
	// will be set to infinite until either they refer to a job that no longer
	// exists or the job fails again.
	Release()

	// Result is used to collect metadata from the step. Usually this is
	// `Success`, but some steps support more types (e.g. `VersionInfo`).
	//
	// Result returns a bool indicating whether it was able to populate the
	// destination. If the destination's type is unknown to the step, it must
	// return false.
	//
	// Implementers of this method MUST not mutate the given pointer if they
	// are unable to respond (i.e. returning false from this function).
	Result(interface{}) bool
}

// Success indicates whether a step completed successfully.
type Success bool

// ExitStatus is the resulting exit code from the process that the step ran.
// Typically if the ExitStatus result is 0, the Success result is true.
type ExitStatus int

// VersionInfo is the version and metadata of a resource that was fetched or
// produced. It is used by Put, Get, and DependentGet.
type VersionInfo struct {
	Version  atc.Version
	Metadata []atc.MetadataField
}

// NoopStep implements a step that successfully does nothing.
type NoopStep struct{}

// Run returns nil immediately without doing anything. It does not bother
// indicating that it's ready or listening for signals.
func (NoopStep) Run(<-chan os.Signal, chan<- struct{}) error {
	return nil
}

// Release does nothing as there are no resources consumed by the NoopStep.
func (NoopStep) Release() {}

// Result returns false. Arguably it should at least be able to indicate
// Success (as true), though it currently does not.
func (NoopStep) Result(interface{}) bool {
	return false
}
