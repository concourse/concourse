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
}

type Stepper func(atc.Plan) Step

func NewRunState(
	stepper Stepper,
	credVars vars.Variables,
	enableRedaction bool,
) RunState {
	return &runState{
		stepper: stepper,

		vars: newBuildVariables(credVars, enableRedaction),

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

func (state *runState) Get(ref vars.Reference) (interface{}, bool, error) {
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
	return &clone
}

func (state *runState) Parent() RunState {
	return state.parent
}

func (state *runState) AddLocalVar(name string, val interface{}, redact bool) {
	state.vars.AddLocalVar(name, val, redact)
}

func (state *runState) RedactionEnabled() bool {
	return state.vars.RedactionEnabled()
}

func (state *runState) Run(ctx context.Context, plan atc.Plan) (bool, error) {
	return state.stepper(plan).Run(ctx, state)
}
