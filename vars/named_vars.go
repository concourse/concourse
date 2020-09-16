package vars

type NamedVariables map[string]Variables

// Get checks var_source if presents, then forward var to underlying secret manager.
// A `varDef.Name` with a var_source looks like "myvault:foo", where "myvault" is
// the var_source name, and "foo" is the real var name that should be forwarded
// to the underlying secret manager.
func (m NamedVariables) Get(varDef VariableDefinition) (interface{}, bool, error) {
	if varDef.Ref.Source == "" {
		return nil, false, nil
	}

	if vars, ok := m[varDef.Ref.Source]; ok {
		return vars.Get(varDef)
	}

	return nil, false, MissingSourceError{Name: varDef.Ref.Name, Source: varDef.Ref.Source}
}

func (m NamedVariables) List() ([]VariableDefinition, error) {
	var allDefs []VariableDefinition

	for source, vars := range m {
		defs, err := vars.List()
		if err != nil {
			return nil, err
		}

		for i, _ := range defs {
			defs[i].Ref.Source = source
		}

		allDefs = append(allDefs, defs...)
	}

	return allDefs, nil
}
