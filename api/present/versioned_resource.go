package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
)

func VersionedResource(vr dbng.VersionedResource) atc.VersionedResource {
	return atc.VersionedResource{
		Resource: vr.Resource,
		Version:  atc.Version(vr.Version),
	}
}

func SavedVersionedResource(svr dbng.SavedVersionedResource) atc.VersionedResource {
	var metadata []atc.MetadataField

	for _, v := range svr.Metadata {
		metadata = append(metadata, atc.MetadataField{
			Name:  v.Name,
			Value: v.Value,
		})
	}

	return atc.VersionedResource{
		ID:       svr.ID,
		Resource: svr.Resource,
		Enabled:  svr.Enabled,
		Type:     svr.Type,
		Version:  atc.Version(svr.Version),
		Metadata: metadata,
	}
}
