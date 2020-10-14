package vars

type StaticVariables map[string]interface{}

var _ Variables = StaticVariables{}

func (v StaticVariables) Get(ref Reference) (interface{}, bool, error) {
	val, found := v[ref.Path]
	if !found {
		return nil, false, nil
	}
	val, err := Traverse(val, ref.String(), ref.Fields)
	if err != nil {
		return nil, false, err
	}
	return val, true, nil
}

func (v StaticVariables) List() ([]Reference, error) {
	var refs []Reference

	for name, _ := range v {
		refs = append(refs, Reference{Path: name})
	}

	return refs, nil
}

func Traverse(val interface{}, name string, fields []string) (interface{}, error) {
	for _, seg := range fields {
		switch v := val.(type) {
		case map[interface{}]interface{}:
			var found bool
			val, found = v[seg]
			if !found {
				return nil, MissingFieldError{
					Name:  name,
					Field: seg,
				}
			}
		case map[string]interface{}:
			var found bool
			val, found = v[seg]
			if !found {
				return nil, MissingFieldError{
					Name:  name,
					Field: seg,
				}
			}
		default:
			return nil, InvalidFieldError{
				Name:  name,
				Field: seg,
				Value: val,
			}
		}
	}
	return val, nil
}
