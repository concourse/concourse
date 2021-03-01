package vars

type NamedVariables map[string]Variables

// Get checks var_source if presents, then forward var to underlying secret manager.
// A Reference with a var_source looks like "myvault:foo", where "myvault" is
// the var_source name, and "foo" is the real var name that should be forwarded
// to the underlying secret manager.
func (m NamedVariables) Get(ref Reference) (interface{}, bool, error) {
	if ref.Source == "" {
		return nil, false, nil
	}

	if vars, ok := m[ref.Source]; ok {
		return vars.Get(ref.WithoutSource())
	}

	return nil, false, MissingSourceError{Name: ref.String(), Source: ref.Source}
}

func (m NamedVariables) List() ([]Reference, error) {
	var allRefs []Reference

	for source, vars := range m {
		refs, err := vars.List()
		if err != nil {
			return nil, err
		}

		for i, _ := range refs {
			refs[i].Source = source
		}

		allRefs = append(allRefs, refs...)
	}

	return allRefs, nil
}
