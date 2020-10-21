package atc

type VersionedResourceType struct {
	ResourceType

	Version Version `json:"version"`
}

type VersionedResourceTypes []VersionedResourceType

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

func (types VersionedResourceTypes) Base(name string) string {
	base := name
	for {
		resourceType, found := types.Lookup(base)
		if !found {
			break
		}

		types = types.Without(base)
		base = resourceType.Type
	}

	return base
}
