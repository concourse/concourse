package volume

type Properties map[string]string

func (p Properties) HasProperties(other Properties) bool {
	if len(other) > len(p) {
		return false
	}

	for otherName, otherValue := range other {
		value, found := p[otherName]
		if !found || value != otherValue {
			return false
		}
	}

	return true
}

func (p Properties) UpdateProperty(name string, value string) Properties {
	updatedProperties := Properties{}

	for k, v := range p {
		updatedProperties[k] = v
	}

	updatedProperties[name] = value

	return updatedProperties
}
