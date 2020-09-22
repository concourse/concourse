package atc

var baseResourceTypeDefaults = map[string]Source{}

func LoadBaseResourceTypeDefaults(defaults map[string]Source) {
	baseResourceTypeDefaults = defaults
}

func FindBaseResourceTypeDefaults(name string) (Source, bool) {
	if source, ok := baseResourceTypeDefaults[name]; ok {
		return source, true
	}
	return nil, false
}
