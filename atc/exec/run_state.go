package exec

import (
	"context"
	"reflect"
	"sync"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/vars"
)

type runState struct {
	stepper Stepper

	vars *buildVariables

	artifacts *build.Repository
	results   *sync.Map

	parent RunState

	pinnedWorker *pinnedWorker
}

type pinnedWorker struct {
	mu   sync.Mutex
	name string
}

type Stepper func(atc.Plan) Step

func NewRunState(
	stepper Stepper,
	credVars vars.Variables,
) RunState {
	return &runState{
		stepper: stepper,

		vars: newBuildVariables(credVars),

		artifacts: build.NewRepository(),
		results:   &sync.Map{},

		pinnedWorker: &pinnedWorker{},
	}
}

func (state *runState) ArtifactRepository() *build.Repository {
	return state.artifacts
}

func (state *runState) Result(id atc.PlanID, to any) bool {
	val, ok := state.results.Load(id)
	if !ok {
		return false
	}

	if reflect.TypeOf(val).AssignableTo(reflect.TypeOf(to).Elem()) {
		reflect.ValueOf(to).Elem().Set(reflect.ValueOf(val))
		return true
	}

	return false
}

func (state *runState) StoreResult(id atc.PlanID, val any) {
	state.results.Store(id, val)
}

func (state *runState) Get(ref vars.Reference) (any, bool, error) {
	return state.vars.Get(ref)
}

func (state *runState) List() ([]vars.Reference, error) {
	return state.vars.List()
}

func (state *runState) IterateInterpolatedCreds(iter vars.TrackedVarsIterator) {
	state.vars.IterateInterpolatedCreds(iter)
}

func (state *runState) NewLocalScope() RunState {
	clone := *state
	clone.vars = state.vars.NewLocalScope()
	clone.artifacts = state.artifacts.NewLocalScope()
	clone.parent = state
	// Share the build's pinned worker with the new scope so across
	// substeps remain on the same worker as the rest of the build. The
	// embedded mutex is safe to share because SetPinnedWorker is
	// idempotent and only writes when the pinned worker is empty.
	return &clone
}

func (state *runState) Parent() RunState {
	return state.parent
}

func (state *runState) AddLocalVar(name string, val any, redact bool) {
	state.vars.AddLocalVar(name, val, redact)
}

func (state *runState) Run(ctx context.Context, plan atc.Plan) (bool, error) {
	return state.stepper(plan).Run(ctx, state)
}

func (state *runState) PinnedWorker() string {
	state.pinnedWorker.mu.Lock()
	defer state.pinnedWorker.mu.Unlock()
	return state.pinnedWorker.name
}

// SetPinnedWorker records the worker that subsequent steps in the build
// should be pinned to. The check-and-set is atomic, so when two steps
// race to pin a worker, exactly one of them will see their own name
// returned and the other will see the winner's name. If a worker has
// already been pinned, the new value is ignored and the existing
// pinned worker is returned. This guarantees the build only ever pins
// to a single worker.
func (state *runState) SetPinnedWorker(name string) string {
	state.pinnedWorker.mu.Lock()
	defer state.pinnedWorker.mu.Unlock()
	if state.pinnedWorker.name == "" {
		state.pinnedWorker.name = name
	}
	return state.pinnedWorker.name
}
