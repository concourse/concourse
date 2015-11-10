package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

func VersionedResource(vr db.SavedVersionedResource) atc.VersionedResource {
	return atc.VersionedResource{
		Resource: vr.Resource,
		Version:  atc.Version(vr.Version),
	}
}
