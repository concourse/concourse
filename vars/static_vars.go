package vars

type StaticVariables map[string]interface{}

var _ Variables = StaticVariables{}

func (v StaticVariables) Get(varDef VariableDefinition) (interface{}, bool, error) {
	val, found := v[varDef.Ref.Path]
	return val, found, nil
}

func (v StaticVariables) List() ([]VariableDefinition, error) {
	var defs []VariableDefinition

	for name, _ := range v {
		defs = append(defs, VariableDefinition{Ref: VariableReference{Path: name}})
	}

	return defs, nil
}
