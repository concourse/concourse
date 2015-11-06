package present

import (
	"strconv"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web"
	"github.com/tedsuo/rata"
)

func Build(build db.Build) atc.Build {
	var err error
	var reqURL string
	if build.JobName == "" && build.PipelineName == "" {
		reqURL, err = web.Routes.CreatePathForRoute(
			web.GetJoblessBuild,
			rata.Params{"build_id": strconv.Itoa(build.ID)},
		)
	} else {
		reqURL, err = web.Routes.CreatePathForRoute(
			web.GetBuild,
			rata.Params{"job": build.JobName, "build": build.Name, "pipeline_name": build.PipelineName},
		)
	}
	if err != nil {
		panic("failed to generate url: " + err.Error())
	}

	apiURL, err := atc.Routes.CreatePathForRoute(atc.GetBuild, rata.Params{"build_id": strconv.Itoa(build.ID)})
	if err != nil {
		panic("failed to generate url: " + err.Error())
	}

	return atc.Build{
		ID:           build.ID,
		Name:         build.Name,
		Status:       string(build.Status),
		JobName:      build.JobName,
		PipelineName: build.PipelineName,
		URL:          reqURL,
		APIURL:       apiURL,
	}
}
