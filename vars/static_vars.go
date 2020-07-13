package vars

type StaticVariables map[string]interface{}

var _ Variables = StaticVariables{}

func (v StaticVariables) Get(varDef VariableDefinition) (interface{}, bool, error) {
	name := varDef.Ref.Name

	val, found := v[varDef.Ref.Path]
	if !found {
		return val, found, nil
	}

	for _, seg := range varDef.Ref.Fields {
		switch v := val.(type) {
		case map[interface{}]interface{}:
			var found bool
			val, found = v[seg]
			if !found {
				return nil, false, MissingFieldError{
					Name:  name,
					Field: seg,
				}
			}
		case map[string]interface{}:
			var found bool
			val, found = v[seg]
			if !found {
				return nil, false, MissingFieldError{
					Name:  name,
					Field: seg,
				}
			}
		default:
			return nil, false, InvalidFieldError{
				Name:  name,
				Field: seg,
				Value: val,
			}
		}
	}

	return val, found, nil
}

func (v StaticVariables) List() ([]VariableDefinition, error) {
	var defs []VariableDefinition

	for name, _ := range v {
		defs = append(defs, VariableDefinition{Ref: VariableReference{Path: name}})
	}

	return defs, nil
}
