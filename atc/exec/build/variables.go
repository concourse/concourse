package build

import (
	"sync"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/vars"
)

type Variables struct {
	parentScope interface {
		vars.Variables
		IterateInterpolatedCreds(iter vars.TrackedVarsIterator)
	}

	vars    map[string]vars.StaticVariables
	tracker *vars.Tracker

	lock sync.RWMutex

	// source configurations of all the var sources within the pipeline
	sources atc.VarSourceConfigs
}

func NewVariables(sources atc.VarSourceConfigs, enableRedaction bool) *Variables {
	return &Variables{
		vars:    map[string]vars.StaticVariables{},
		tracker: vars.NewTracker(enableRedaction),

		sources: sources,
	}
}

func (v *Variables) VarSourceConfigs() atc.VarSourceConfigs {
	return v.sources
}

func (v *Variables) Get(ref vars.Reference) (interface{}, bool, error) {
	v.lock.RLock()
	defer v.lock.RUnlock()

	source, found := v.vars[ref.Source]
	if found {
		val, found, err := source.Get(ref)
		if found || err != nil {
			return val, found, err
		}
	}

	if v.parentScope != nil {
		val, found, err := v.parentScope.Get(ref)
		if found || err != nil {
			return val, found, err
		}
	}

	return nil, false, nil
}

func (v *Variables) IterateInterpolatedCreds(iter vars.TrackedVarsIterator) {
	v.tracker.IterateInterpolatedCreds(iter)
}

func (v *Variables) NewScope() *Variables {
	return &Variables{
		parentScope: v,
		vars:        map[string]vars.StaticVariables{},
		tracker:     vars.NewTracker(v.tracker.Enabled),
	}
}

// TODO: Add setting a var with fields
func (v *Variables) SetVar(source, name string, val interface{}, redact bool) {
	v.lock.RLock()
	defer v.lock.RUnlock()

	scope, found := v.vars[source]
	if !found {
		scope = vars.StaticVariables{}
		v.vars[source] = scope
	}

	scope[name] = val
	if redact {
		v.tracker.Track(vars.Reference{Source: source, Path: name}, val)
	}
}

func (v *Variables) RedactionEnabled() bool {
	return v.tracker.Enabled
}
