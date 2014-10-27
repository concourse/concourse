package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/builds"
)

func Build(build builds.Build) atc.Build {
	return atc.Build{
		ID:      build.ID,
		Name:    build.Name,
		Status:  string(build.Status),
		JobName: build.JobName,
	}
}
