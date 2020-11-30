package exec

import (
	"github.com/concourse/concourse/vars"
)

type buildVariables struct {
	parentScope interface {
		vars.Variables
		IterateInterpolatedCreds(iter vars.TrackedVarsIterator)
	}

	sourceVars map[string]vars.StaticVariables
	tracker    *vars.Tracker
}

func newBuildVariables(credVars vars.Variables, enableRedaction bool) *buildVariables {
	return &buildVariables{
		parentScope: &vars.CredVarsTracker{
			CredVars: credVars,
			Tracker:  vars.NewTracker(enableRedaction),
		},
		sourceVars: map[string]vars.StaticVariables{},
		tracker:    vars.NewTracker(enableRedaction),
	}
}

func (b *buildVariables) Get(ref vars.Reference) (interface{}, bool, error) {
	source, found := b.sourceVars[ref.Source]
	if found {
		val, found, err := source.Get(ref)
		if found || err != nil {
			return val, found, err
		}
	}

	return b.parentScope.Get(ref)
}

func (b *buildVariables) List() ([]vars.Reference, error) {
	list, err := b.parentScope.List()
	if err != nil {
		return nil, err
	}
	for source, vs := range b.sourceVars {
		for k := range vs {
			list = append(list, vars.Reference{Source: source, Path: k})
		}
	}
	return list, nil
}

func (b *buildVariables) IterateInterpolatedCreds(iter vars.TrackedVarsIterator) {
	b.tracker.IterateInterpolatedCreds(iter)
	b.parentScope.IterateInterpolatedCreds(iter)
}

func (b *buildVariables) NewScope() *buildVariables {
	return &buildVariables{
		parentScope: b,
		sourceVars:  map[string]vars.StaticVariables{},
		tracker:     vars.NewTracker(b.tracker.Enabled),
	}
}

func (b *buildVariables) AddVar(source, name string, val interface{}, redact bool) {
	scope, found := b.sourceVars[source]
	if !found {
		scope = vars.StaticVariables{}
		b.sourceVars[source] = scope
	}

	scope[name] = val
	if redact {
		b.tracker.Track(vars.Reference{Source: source, Path: name}, val)
	}
}

func (b *buildVariables) RedactionEnabled() bool {
	return b.tracker.Enabled
}
