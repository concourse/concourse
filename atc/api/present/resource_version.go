package present

import (
	"github.com/concourse/concourse/v5/atc"
)

func ResourceVersions(hideMetadata bool, resourceVersions []atc.ResourceVersion) []atc.ResourceVersion {
	var presented []atc.ResourceVersion

	for _, resourceVersion := range resourceVersions {
		if hideMetadata {
			resourceVersion.Metadata = nil
		}

		presented = append(presented, resourceVersion)
	}

	return presented
}
