package vars

type StaticVariables map[string]interface{}

var _ Variables = StaticVariables{}

func (v StaticVariables) Get(ref Reference) (interface{}, bool, error) {
	val, found := v[ref.Path]
	return val, found, nil
}

func (v StaticVariables) List() ([]Reference, error) {
	var refs []Reference

	for name, _ := range v {
		refs = append(refs, Reference{Path: name})
	}

	return refs, nil
}
