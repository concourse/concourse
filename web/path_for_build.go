package web

import (
	"fmt"

	"github.com/concourse/atc"
	"github.com/tedsuo/rata"
)

func PathForBuild(build atc.Build) string {
	var path string
	if build.OneOff() {
		path, _ = Routes.CreatePathForRoute(GetJoblessBuild, rata.Params{
			"team_name": build.TeamName,
			"build_id":  fmt.Sprintf("%d", build.ID),
		})
	} else {
		path, _ = Routes.CreatePathForRoute(GetBuild, rata.Params{
			"team_name":     build.TeamName,
			"pipeline_name": build.PipelineName,
			"job":           build.JobName,
			"build":         build.Name,
		})
	}

	return path
}
