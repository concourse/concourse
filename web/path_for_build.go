package web

import (
	"fmt"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/tedsuo/rata"
)

func PathForBuildNew(build atc.Build) string {
	var path string
	if build.OneOff() {
		path, _ = Routes.CreatePathForRoute(GetJoblessBuild, rata.Params{
			"build_id": fmt.Sprintf("%d", build.ID),
		})
	} else {
		path, _ = Routes.CreatePathForRoute(GetBuild, rata.Params{
			"pipeline_name": build.PipelineName,
			"job":           build.JobName,
			"build":         build.Name,
		})
	}

	return path
}

func PathForBuild(build db.Build) string {
	var path string
	if build.OneOff() {
		path, _ = Routes.CreatePathForRoute(GetJoblessBuild, rata.Params{
			"build_id": fmt.Sprintf("%d", build.ID),
		})
	} else {
		path, _ = Routes.CreatePathForRoute(GetBuild, rata.Params{
			"pipeline_name": build.PipelineName,
			"job":           build.JobName,
			"build":         build.Name,
		})
	}

	return path
}

func PathForATCBuild(build atc.Build) string {
	var path string
	if build.JobName == "" {
		path, _ = Routes.CreatePathForRoute(GetJoblessBuild, rata.Params{
			"build_id": fmt.Sprintf("%d", build.ID),
		})
	} else {
		path, _ = Routes.CreatePathForRoute(GetBuild, rata.Params{
			"pipeline_name": build.PipelineName,
			"job":           build.JobName,
			"build":         build.Name,
		})
	}

	return path
}
