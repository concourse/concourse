package creds

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/vars"
)

type VersionedResourceType struct {
	atc.VersionedResourceType

	Source Source
}

type VersionedResourceTypes []VersionedResourceType

func NewVersionedResourceTypes(variables vars.Variables, rawTypes atc.VersionedResourceTypes) VersionedResourceTypes {
	var types VersionedResourceTypes
	for _, t := range rawTypes {
		types = append(types, VersionedResourceType{
			VersionedResourceType: t,
			Source:                NewSource(variables, t.Source),
		})
	}

	return types
}

func (types VersionedResourceTypes) Lookup(name string) (VersionedResourceType, bool) {
	for _, t := range types {
		if t.Name == name {
			return t, true
		}
	}

	return VersionedResourceType{}, false
}

func (types VersionedResourceTypes) Without(name string) VersionedResourceTypes {
	newTypes := VersionedResourceTypes{}

	for _, t := range types {
		if t.Name != name {
			newTypes = append(newTypes, t)
		}
	}

	return newTypes
}

func (types VersionedResourceTypes) Evaluate() (atc.VersionedResourceTypes, error) {

	var rawTypes atc.VersionedResourceTypes
	for _, t := range types {
		source, err := t.Source.Evaluate()
		if err != nil {
			return nil, err
		}

		resourceType := t.ResourceType
		resourceType.Source = source

		rawTypes = append(rawTypes, atc.VersionedResourceType{
			ResourceType: resourceType,
			Version:      t.Version,
		})
	}

	return rawTypes, nil
}
