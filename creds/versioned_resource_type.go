package creds

import "github.com/concourse/atc"

type VersionedResourceType struct {
	atc.VersionedResourceType

	Source Source
}

type VersionedResourceTypes []VersionedResourceType

func NewVersionedResourceTypes(variables Variables, rawTypes atc.VersionedResourceTypes) VersionedResourceTypes {
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
