package build

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/vars"
)

type Variables struct {
	vars    map[string]vars.StaticVariables
	tracker *vars.Tracker

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

func (v *Variables) VarSources() atc.VarSourceConfigs {
	return v.sources
}

func (v *Variables) Get(ref vars.Reference) (interface{}, bool, error) {
	source, found := v.vars[ref.Source]
	if found {
		val, found, err := source.Get(ref)
		if found || err != nil {
			return val, found, err
		}
	}

	return nil, false, nil
}

func (b *Variables) IterateInterpolatedCreds(iter vars.TrackedVarsIterator) {
	b.tracker.IterateInterpolatedCreds(iter)
}

func (b *Variables) NewScope() *Variables {
	return &Variables{
		vars:    map[string]vars.StaticVariables{},
		tracker: vars.NewTracker(b.tracker.Enabled),
	}
}

func (b *Variables) SetVar(source, name string, val interface{}, redact bool) {
	scope, found := b.vars[source]
	if !found {
		scope = vars.StaticVariables{}
		b.vars[source] = scope
	}

	scope[name] = val
	if redact {
		b.tracker.Track(vars.Reference{Source: source, Path: name}, val)
	}
}

func (b *Variables) RedactionEnabled() bool {
	return b.tracker.Enabled
}
