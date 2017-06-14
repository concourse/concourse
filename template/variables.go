package template

type Variables map[string]string

func (v Variables) Merge(other Variables) Variables {
	merged := Variables{}

	for key, value := range v {
		merged[key] = value
	}

	for key, value := range other {
		merged[key] = value
	}

	return merged
}
