package exec

import (
	"reflect"
	"sync"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/exec/build"
)

type runState struct {
	artifacts *build.Repository
	results   *sync.Map
}

func NewRunState() RunState {
	return &runState{
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
