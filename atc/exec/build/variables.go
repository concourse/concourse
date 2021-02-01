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

	localVars vars.StaticVariables
	tracker   *vars.Tracker

	lock sync.RWMutex
}

func NewVariables(sources atc.VarSourceConfigs, tracker *vars.Tracker) *Variables {
	return &Variables{
		localVars: vars.StaticVariables{},
		tracker:   tracker,
	}
}

func (v *Variables) Get(ref vars.Reference) (interface{}, bool, error) {
	if ref.Source == "." {
		v.lock.RLock()
		val, found, err := v.localVars.Get(ref.WithoutSource())
		v.lock.RUnlock()
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
		localVars:   vars.StaticVariables{},
		tracker:     vars.NewTracker(v.tracker.Enabled),
	}
}

func (v *Variables) SetVar(source, name string, val interface{}, redact bool) {
	v.lock.RLock()
	v.localVars[name] = val
	v.lock.RUnlock()

	if redact {
		v.tracker.Track(vars.Reference{Source: source, Path: name}, val)
	}
}

func (v *Variables) RedactionEnabled() bool {
	return v.tracker.Enabled
}
