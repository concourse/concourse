package present

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func VersionedResourceTypes(showCheckError bool, savedResourceTypes db.ResourceTypes) atc.VersionedResourceTypes {
	versionedResourceTypes := savedResourceTypes.Deserialize()

	for i, resourceType := range savedResourceTypes {
		if resourceType.CheckSetupError() != nil && showCheckError {
			versionedResourceTypes[i].CheckSetupError = resourceType.CheckSetupError().Error()
		} else {
			versionedResourceTypes[i].CheckSetupError = ""
		}

		if resourceType.CheckError() != nil && showCheckError {
			versionedResourceTypes[i].CheckError = resourceType.CheckError().Error()
		} else {
			versionedResourceTypes[i].CheckError = ""
		}
	}

	return versionedResourceTypes
}
