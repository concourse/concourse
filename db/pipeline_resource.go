package db

import (
	"time"

	"github.com/concourse/atc"
)

type VersionedResource struct {
	Resource string
	Type     string
	Version  ResourceVersion
	Metadata ResourceMetadataFields
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

type ResourceMetadataFields []ResourceMetadataField

func NewResourceMetadataFields(atcm []atc.MetadataField) ResourceMetadataFields {
	metadata := make([]ResourceMetadataField, len(atcm))
	for i, md := range atcm {
		metadata[i] = ResourceMetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	return metadata
}

func (rmf ResourceMetadataFields) ToATCMetadata() []atc.MetadataField {
	metadata := make([]atc.MetadataField, len(rmf))
	for i, md := range rmf {
		metadata[i] = atc.MetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	return metadata
}

type BuildInput struct {
	Name string

	VersionedResource

	FirstOccurrence bool
}

type BuildOutput struct {
	VersionedResource
}
