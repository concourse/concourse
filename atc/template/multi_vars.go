package template

type MultiVars struct {
	varss []Variables
}

func NewMultiVars(varss []Variables) MultiVars {
	return MultiVars{varss}
}

var _ Variables = MultiVars{}

func (m MultiVars) Get(varDef VariableDefinition) (interface{}, bool, error) {
	for _, vars := range m.varss {
		val, found, err := vars.Get(varDef)
		if found || err != nil {
			return val, found, err
		}
	}

	return nil, false, nil
}

func (m MultiVars) List() ([]VariableDefinition, error) {
	var allDefs []VariableDefinition

	for _, vars := range m.varss {
		defs, err := vars.List()
		if err != nil {
			return nil, err
		}

		allDefs = append(allDefs, defs...)
	}

	return allDefs, nil
}
