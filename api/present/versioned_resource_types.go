package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

func VersionedResourceTypes(savedResourceTypes db.ResourceTypes) atc.VersionedResourceTypes {
	versionedResourceTypes := savedResourceTypes.Deserialize()
	return versionedResourceTypes
}
