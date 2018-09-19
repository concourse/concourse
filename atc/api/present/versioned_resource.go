package present

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func ResourceConfigVersion(o db.BuildOutput) atc.VersionedResource {
	return atc.VersionedResource{
		Resource: o.Name,
		Version:  atc.Version(o.Version),
	}
}
