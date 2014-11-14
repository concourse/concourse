package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web/routes"
	"github.com/tedsuo/rata"
)

func Build(build db.Build) atc.Build {
	generator := rata.NewRequestGenerator("", routes.Routes)

	req, err := generator.CreateRequest(
		routes.GetBuild,
		rata.Params{"job": build.JobName, "build": build.Name},
		nil,
	)
	if err != nil {
		panic("failed to generate url: " + err.Error())
	}

	return atc.Build{
		ID:      build.ID,
		Name:    build.Name,
		Status:  string(build.Status),
		JobName: build.JobName,
		URL:     req.URL.String(),
	}
}
