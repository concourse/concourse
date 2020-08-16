package vars

type BuildVariables struct {
	parentScope interface {
		Variables
		IterateInterpolatedCreds(iter TrackedVarsIterator)
	}

	localVars StaticVariables
	tracker   *tracker
}

func NewBuildVariables(credVars Variables, enableRedaction bool) *BuildVariables {
	return &BuildVariables{
		parentScope: &credVarsTracker{
			credVars: credVars,
			tracker:  newTracker(enableRedaction),
		},
		localVars: StaticVariables{},
		tracker:   newTracker(enableRedaction),
	}
}

func (b *BuildVariables) Get(varDef VariableDefinition) (interface{}, bool, error) {
	if varDef.Ref.Source == "." {
		val, found, _ := b.localVars.Get(varDef)
		if found {
			return val, true, nil
		}
	}
	return b.parentScope.Get(varDef)
}

func (b *BuildVariables) List() ([]VariableDefinition, error) {
	list, err := b.parentScope.List()
	if err != nil {
		return nil, err
	}
	for k := range b.localVars {
		list = append(list, VariableDefinition{
			Ref: VariableReference{Source: ".", Path: k},
		})
	}
	return list, nil
}

func (b *BuildVariables) IterateInterpolatedCreds(iter TrackedVarsIterator) {
	b.tracker.IterateInterpolatedCreds(iter)
	b.parentScope.IterateInterpolatedCreds(iter)
}

func (b *BuildVariables) NewLocalScope() *BuildVariables {
	return &BuildVariables{
		parentScope: b,
		localVars:   StaticVariables{},
		tracker:     newTracker(b.tracker.enabled),
	}
}

func (b *BuildVariables) AddLocalVar(name string, val interface{}, redact bool) {
	b.localVars[name] = val
	if redact {
		b.tracker.Track(VariableReference{Source: ".", Path: name}, val)
	}
}

func (b *BuildVariables) RedactionEnabled() bool {
	return b.tracker.enabled
}
