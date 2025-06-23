package volume

import "maps"

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
	updatedProperties := maps.Clone(p)

	updatedProperties[name] = value

	return updatedProperties
}
