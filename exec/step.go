package exec

import (
	"context"

	"github.com/concourse/atc"
	"github.com/concourse/atc/worker"
)

//go:generate counterfeiter . StepFactory

// StepFactory constructs a step. The previous step and source repository are
// provided.
//
// Some steps, i.e. DependentGetStep, use information from the previous step to
// determine how to run.
type StepFactory interface {
	Using(*worker.ArtifactRepository) Step
}

//go:generate counterfeiter . Step

// A Step is an object that can be executed, whose result (e.g. Success) can be
// collected, and whose dependent resources (e.g. Containers, Volumes) can be
// released, allowing them to expire.
type Step interface {
	// Run is called when it's time to execute the step. It should watch for the
	// given context to be canceled in the event that the build is aborted or the
	// step times out, and be sure to propagate the (context.Context).Err().
	//
	// Steps wrapping other steps should be careful to propagate the context.
	//
	// Steps must be idempotent. Each step is responsible for handling its own
	// idempotency.
	Run(context.Context) error

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
