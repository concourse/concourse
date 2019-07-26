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
