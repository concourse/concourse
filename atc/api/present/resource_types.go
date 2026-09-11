package present

import (
	"github.com/concourse/concourse/v8/atc"
	"github.com/concourse/concourse/v8/atc/db"
)

func ResourceTypes(savedResourceTypes db.ResourceTypes) atc.ResourceTypes {
	return savedResourceTypes.Deserialize()
}
