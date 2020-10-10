package present

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func VersionedResourceTypes(savedResourceTypes db.ResourceTypes) atc.VersionedResourceTypes {
	return savedResourceTypes.Deserialize()
}
