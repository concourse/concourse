package exec

import (
	"context"
	"io"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/vars"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . Step

// A Step is an object that can be executed, whose result (e.g. Success) can be
// collected, and whose dependent resources (e.g. Containers, Volumes) can be
// released, allowing them to expire.
type Step interface {
	// Run executes the step, returning true if the step succeeds, false if it
	// fails, or an error if an error occurs.
	//
	// Run should watch for the given context to be canceled in the event that
	// the build is aborted or the step times out, and be sure to propagate the
	// (context.Context).Err().
	//
	// Steps wrapping other steps should be careful to propagate the context.
	//
	// Steps must be idempotent. Each step is responsible for handling its own
	// idempotency.
	Run(context.Context, RunState) (bool, error)
}

//counterfeiter:generate . BuildStepDelegate
type BuildOutputFilter func(text string) string

//counterfeiter:generate . RunState
type RunState interface {
	vars.Variables

	NewLocalScope() RunState
	AddLocalVar(name string, val interface{}, redact bool)

	IterateInterpolatedCreds(vars.TrackedVarsIterator)
	RedactionEnabled() bool

	ArtifactRepository() *build.Repository

	Result(atc.PlanID, interface{}) bool
	StoreResult(atc.PlanID, interface{})

	Run(context.Context, atc.Plan) (bool, error)

	Parent() RunState
}

// ExitStatus is the resulting exit code from the process that the step ran.
// Typically if the ExitStatus result is 0, the Success result is true.
type ExitStatus int

// Privileged is used to indicate whether the given step should run with
// special privileges (i.e. as an administrator user).
type Privileged bool

type InputHandler func(io.ReadCloser) error
type OutputHandler func(io.Writer) error

//go:generate counterfeiter . Pool

type Pool interface {
	FindOrSelectWorker(context.Context, db.ContainerOwner, runtime.ContainerSpec, worker.Spec, worker.PlacementStrategy, worker.PoolCallback) (runtime.Worker, error)
	FindResourceCacheVolume(lager.Logger, int, db.ResourceCache, worker.Spec) (runtime.Volume, bool, error)
	FindResourceCacheVolumeOnWorker(lager.Logger, db.ResourceCache, worker.Spec, string) (runtime.Volume, bool, error)
	ReleaseWorker(lager.Logger, runtime.ContainerSpec, runtime.Worker, worker.PlacementStrategy)
	LocateVolume(logger lager.Logger, teamID int, handle string) (runtime.Volume, runtime.Worker, bool, error)
}

//go:generate counterfeiter . Streamer

type Streamer interface {
	StreamFile(ctx context.Context, artifact runtime.Artifact, path string) (io.ReadCloser, error)
}
