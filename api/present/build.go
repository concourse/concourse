package present

import (
	"strconv"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web"
	"github.com/tedsuo/rata"
)

func Build(build db.BuildDB) atc.Build {
	var err error
	var reqURL string
	if build.GetJobName() == "" && build.GetPipelineName() == "" {
		reqURL, err = web.Routes.CreatePathForRoute(
			web.GetJoblessBuild,
			rata.Params{
				"build_id":  strconv.Itoa(build.GetID()),
				"team_name": build.GetTeamName(),
			},
		)
	} else {
		reqURL, err = web.Routes.CreatePathForRoute(
			web.GetBuild,
			rata.Params{
				"job":           build.GetJobName(),
				"build":         build.GetName(),
				"pipeline_name": build.GetPipelineName(),
				"team_name":     build.GetTeamName(),
			},
		)
	}
	if err != nil {
		panic("failed to generate url: " + err.Error())
	}

	apiURL, err := atc.Routes.CreatePathForRoute(atc.GetBuild, rata.Params{
		"build_id":  strconv.Itoa(build.GetID()),
		"team_name": build.GetTeamName(),
	})
	if err != nil {
		panic("failed to generate url: " + err.Error())
	}

	atcBuild := atc.Build{
		ID:           build.GetID(),
		Name:         build.GetName(),
		Status:       string(build.GetStatus()),
		JobName:      build.GetJobName(),
		PipelineName: build.GetPipelineName(),
		TeamName:     build.GetTeamName(),
		URL:          reqURL,
		APIURL:       apiURL,
	}

	if !build.GetStartTime().IsZero() {
		atcBuild.StartTime = build.GetStartTime().Unix()
	}

	if !build.GetEndTime().IsZero() {
		atcBuild.EndTime = build.GetEndTime().Unix()
	}

	if !build.GetReapTime().IsZero() {
		atcBuild.ReapTime = build.GetReapTime().Unix()
	}

	return atcBuild
}
