package paths

import (
	"fmt"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web/routes"
	"github.com/tedsuo/rata"
)

func PathForBuild(build db.Build) string {
	var path string
	if build.OneOff() {
		path, _ = routes.Routes.CreatePathForRoute(routes.GetJoblessBuild, rata.Params{
			"build_id": fmt.Sprintf("%d", build.ID),
		})
	} else {
		path, _ = routes.Routes.CreatePathForRoute(routes.GetBuild, rata.Params{
			"pipeline_name": build.PipelineName,
			"job":           build.JobName,
			"build":         build.Name,
		})
	}

	return path
}
