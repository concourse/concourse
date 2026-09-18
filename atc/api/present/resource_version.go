package present

import (
	"github.com/concourse/concourse/atc"
)

func ResourceVersions(hideMetadata bool, resourceVersions []atc.ResourceVersion) []atc.ResourceVersion {
	presented := make([]atc.ResourceVersion, 0, len(resourceVersions))

	for _, resourceVersion := range resourceVersions {
		if hideMetadata {
			resourceVersion.Metadata = nil
		}

		presented = append(presented, resourceVersion)
	}

	return presented
}

func ResourceVersion(hideMetadata bool, resourceVersion atc.ResourceVersion) atc.ResourceVersion {
	if hideMetadata {
		resourceVersion.Metadata = nil
	}

	return resourceVersion
}
