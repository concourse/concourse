package exec

import (
	"github.com/concourse/concourse/vars"
)

type buildVariables struct {
	parentScope interface {
		vars.Variables
		IterateInterpolatedCreds(iter vars.TrackedVarsIterator)
	}

	localVars vars.StaticVariables
	tracker   *vars.Tracker
}

func newBuildVariables(credVars vars.Variables, enableRedaction bool) *buildVariables {
	return &buildVariables{
		parentScope: &vars.CredVarsTracker{
			CredVars: credVars,
			Tracker:  vars.NewTracker(enableRedaction),
		},
		localVars: vars.StaticVariables{},
		tracker:   vars.NewTracker(enableRedaction),
	}
}

func (b *buildVariables) Get(ref vars.Reference) (interface{}, bool, error) {
	if ref.Source == "." {
		val, found, _ := b.localVars.Get(ref)
		if found {
			return val, true, nil
		}
	}
	return b.parentScope.Get(ref)
}

func (b *buildVariables) List() ([]vars.Reference, error) {
	list, err := b.parentScope.List()
	if err != nil {
		return nil, err
	}
	for k := range b.localVars {
		list = append(list, vars.Reference{Source: ".", Path: k})
	}
	return list, nil
}

func (b *buildVariables) IterateInterpolatedCreds(iter vars.TrackedVarsIterator) {
	b.tracker.IterateInterpolatedCreds(iter)
	b.parentScope.IterateInterpolatedCreds(iter)
}

func (b *buildVariables) NewLocalScope() *buildVariables {
	return &buildVariables{
		parentScope: b,
		localVars:   vars.StaticVariables{},
		tracker:     vars.NewTracker(b.tracker.Enabled),
	}
}

func (b *buildVariables) AddLocalVar(name string, val interface{}, redact bool) {
	b.localVars[name] = val
	if redact {
		b.tracker.Track(vars.Reference{Source: ".", Path: name}, val)
	}
}

func (b *buildVariables) RedactionEnabled() bool {
	return b.tracker.Enabled
}
