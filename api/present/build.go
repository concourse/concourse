package present

import (
	"github.com/concourse/atc/api/resources"
	"github.com/concourse/atc/builds"
)

func Build(build builds.Build) resources.Build {
	return resources.Build{
		ID:      build.ID,
		Name:    build.Name,
		Status:  string(build.Status),
		JobName: build.JobName,
	}
}
