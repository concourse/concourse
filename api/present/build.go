package present

import (
	"net/http"
	"strconv"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web/routes"
	"github.com/tedsuo/rata"
)

func Build(build db.Build) atc.Build {
	generator := rata.NewRequestGenerator("", routes.Routes)

	var err error
	var req *http.Request
	if build.JobName == "" && build.PipelineName == "" {
		req, err = generator.CreateRequest(
			routes.GetJoblessBuild,
			rata.Params{"build_id": strconv.Itoa(build.ID)},
			nil,
		)
	} else {
		req, err = generator.CreateRequest(
			routes.GetBuild,
			rata.Params{"job": build.JobName, "build": build.Name, "pipeline_name": build.PipelineName},
			nil,
		)
	}
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
