package vars

type FetchedVariables map[string]interface{}

var _ Variables = FetchedVariables{}

func (v FetchedVariables) Get(ref Reference) (interface{}, bool, error) {
	val, found := v[ref.String()]
	if !found {
		return nil, false, nil
	}

	return val, true, nil
}
