package present

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func ResourceTypes(savedResourceTypes db.ResourceTypes) atc.ResourceTypes {
	return savedResourceTypes.Deserialize()
}
