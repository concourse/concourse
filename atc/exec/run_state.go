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

	buildVars *build.Variables

	artifacts *build.Repository
	results   *sync.Map

	// source configurations of all the var sources within the pipeline
	sources atc.VarSourceConfigs

	parent RunState
}

type Stepper func(atc.Plan) Step

func NewRunState(
	stepper Stepper,
	varSourceConfigs atc.VarSourceConfigs,
	enableRedaction bool,
) RunState {
	return &runState{
		stepper: stepper,

		buildVars: build.NewVariables(varSourceConfigs, enableRedaction),

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

func (state *runState) Variables() *build.Variables {
	return state.buildVars
}

func (state *runState) IterateInterpolatedCreds(iter vars.TrackedVarsIterator) {
	state.buildVars.IterateInterpolatedCreds(iter)
}

func (state *runState) NewScope() RunState {
	clone := *state
	clone.buildVars = state.buildVars.NewScope()
	clone.artifacts = state.artifacts.NewScope()
	clone.parent = state
	return &clone
}

func (state *runState) Parent() RunState {
	return state.parent
}

func (state *runState) RedactionEnabled() bool {
	return state.buildVars.RedactionEnabled()
}

func (state *runState) Run(ctx context.Context, plan atc.Plan) (bool, error) {
	return state.stepper(plan).Run(ctx, state)
}

func (state *runState) VarSourceConfigs() atc.VarSourceConfigs {
	return state.sources
}

func (state *runState) SetVarSourceConfigs(varSourceConfigs atc.VarSourceConfigs) {
	state.sources = varSourceConfigs
}
