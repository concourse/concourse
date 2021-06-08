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

	localVars *build.Variables
	tracker   *vars.Tracker

	artifacts *build.Repository
	results   *sync.Map

	parent *runState
}

type Stepper func(atc.Plan) Step

func NewRunState(
	stepper Stepper,
	enableRedaction bool,
) RunState {
	tracker := vars.NewTracker(enableRedaction)
	return &runState{
		stepper: stepper,

		tracker:   tracker,
		localVars: build.NewVariables(tracker),

		artifacts: build.NewRepository(),
		results:   &sync.Map{},
	}
}

func (state *runState) ArtifactRepository() *build.Repository {
	return state.artifacts
}

func (state *runState) Result(id atc.PlanID, to interface{}) bool {
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

func (state *runState) StoreResult(id atc.PlanID, val interface{}) {
	state.results.Store(id, val)
}

func (state *runState) LocalVariables() *build.Variables {
	return state.localVars
}

func (state *runState) IterateInterpolatedCreds(iter vars.TrackedVarsIterator) {
	state.tracker.IterateInterpolatedCreds(iter)
	if state.parent != nil {
		state.parent.tracker.IterateInterpolatedCreds(iter)
	}
}

func (state *runState) Track(ref vars.Reference, value interface{}) {
	state.tracker.Track(ref, value)
}

func (state *runState) NewScope() RunState {
	clone := *state
	clone.tracker = vars.NewTracker(state.tracker.Enabled)
	clone.localVars = state.localVars.NewScope(clone.tracker)
	clone.artifacts = state.artifacts.NewScope()
	clone.parent = state
	return &clone
}

func (state *runState) Parent() RunState {
	return state.parent
}

func (state *runState) RedactionEnabled() bool {
	return state.localVars.RedactionEnabled()
}

func (state *runState) Run(ctx context.Context, plan atc.Plan) (bool, error) {
	return state.stepper(plan).Run(ctx, state)
}
