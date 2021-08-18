package creds

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/vars"
)

type ResourceType struct {
	atc.ResourceType

	Source Source
}

type ResourceTypes []ResourceType

func NewResourceTypes(variables vars.Variables, rawTypes atc.ResourceTypes) ResourceTypes {
	var types ResourceTypes
	for _, t := range rawTypes {
		types = append(types, ResourceType{
			ResourceType: t,
			Source:       NewSource(variables, t.Source),
		})
	}

	return types
}

func (types ResourceTypes) Lookup(name string) (ResourceType, bool) {
	for _, t := range types {
		if t.Name == name {
			return t, true
		}
	}

	return ResourceType{}, false
}

func (types ResourceTypes) Without(name string) ResourceTypes {
	newTypes := ResourceTypes{}

	for _, t := range types {
		if t.Name != name {
			newTypes = append(newTypes, t)
		}
	}

	return newTypes
}

func (types ResourceTypes) Evaluate() (atc.ResourceTypes, error) {
	var rawTypes atc.ResourceTypes
	for _, t := range types {
		source, err := t.Source.Evaluate()
		if err != nil {
			return nil, err
		}

		resourceType := t.ResourceType
		resourceType.Source = source

		rawTypes = append(rawTypes, resourceType)
	}

	return rawTypes, nil
}
