package vars

type MultiVars struct {
	varss []Variables
}

func NewMultiVars(varss []Variables) MultiVars {
	return MultiVars{varss}
}

var _ Variables = MultiVars{}

func (m MultiVars) Get(ref Reference) (interface{}, bool, error) {
	for _, vars := range m.varss {
		val, found, err := vars.Get(ref)
		if found || err != nil {
			return val, found, err
		}
	}

	return nil, false, nil
}

func (m MultiVars) List() ([]Reference, error) {
	var allRefs []Reference

	for _, vars := range m.varss {
		defs, err := vars.List()
		if err != nil {
			return nil, err
		}

		allRefs = append(allRefs, defs...)
	}

	return allRefs, nil
}
