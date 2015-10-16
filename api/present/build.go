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
	var reqUrl string
	if build.JobName == "" && build.PipelineName == "" {
		reqUrl, err = web.Routes.CreatePathForRoute(
			web.GetJoblessBuild,
			rata.Params{"build_id": strconv.Itoa(build.ID)},
		)
	} else {
		reqUrl, err = web.Routes.CreatePathForRoute(
			web.GetBuild,
			rata.Params{"job": build.JobName, "build": build.Name, "pipeline_name": build.PipelineName},
		)
	}
	if err != nil {
		panic("failed to generate url: " + err.Error())
	}

	apiUrl, err := atc.Routes.CreatePathForRoute(atc.GetBuild, rata.Params{"build_id": strconv.Itoa(build.ID)})
	if err != nil {
		panic("failed to generate url: " + err.Error())
	}

	return atc.Build{
		ID:      build.ID,
		Name:    build.Name,
		Status:  string(build.Status),
		JobName: build.JobName,
		URL:     reqUrl,
		ApiUrl:  apiUrl,
	}
}
