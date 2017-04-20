package dbng

import "time"

type VersionedResource struct {
	Resource string
	Type     string
	Version  ResourceVersion
	Metadata []ResourceMetadataField
}

type SavedVersionedResource struct {
	ID           int
	Enabled      bool
	ModifiedTime time.Time
	VersionedResource
	CheckOrder int
}

type SavedVersionedResources []SavedVersionedResource

type ResourceVersion map[string]string

type ResourceMetadataField struct {
	Name  string
	Value string
}

type BuildInput struct {
	Name string

	VersionedResource

	FirstOccurrence bool
}

type BuildOutput struct {
	VersionedResource
}
