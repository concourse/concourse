package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

func Build(build db.Build) atc.Build {
	return atc.Build{
		ID:      build.ID,
		Name:    build.Name,
		Status:  string(build.Status),
		JobName: build.JobName,
	}
}
