package present

import (
	"strconv"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/db"
	"github.com/tedsuo/rata"
)

func Build(build db.Build, job db.Job, access accessor.Access) atc.Build {

	apiURL, err := atc.Routes.CreatePathForRoute(atc.GetBuild, rata.Params{
		"build_id":  strconv.Itoa(build.ID()),
		"team_name": build.TeamName(),
	})
	if err != nil {
		panic("failed to generate url: " + err.Error())
	}

	atcBuild := atc.Build{
		ID:                   build.ID(),
		Name:                 build.Name(),
		JobName:              build.JobName(),
		ResourceName:         build.ResourceName(),
		PipelineID:           build.PipelineID(),
		PipelineName:         build.PipelineName(),
		PipelineInstanceVars: build.PipelineInstanceVars(),
		TeamName:             build.TeamName(),
		Status:               atc.BuildStatus(build.Status()),
		APIURL:               apiURL,
		CreatedBy:            build.CreatedBy(),
	}

	showComments := false
	if job != nil {
		showComments = showComments || job.Public()
	}
	if access != nil {
		showComments = showComments || access.IsAuthorized(build.TeamName())
	}

	if showComments {
		comment, _ := build.Comment()
		atcBuild.Comment = comment
	}

	if build.RerunOf() != 0 {
		atcBuild.RerunNumber = build.RerunNumber()
		atcBuild.RerunOf = &atc.RerunOfBuild{
			Name: build.RerunOfName(),
			ID:   build.RerunOf(),
		}
	}

	if !build.StartTime().IsZero() {
		atcBuild.StartTime = build.StartTime().Unix()
	}

	if !build.EndTime().IsZero() {
		atcBuild.EndTime = build.EndTime().Unix()
	}

	if !build.ReapTime().IsZero() {
		atcBuild.ReapTime = build.ReapTime().Unix()
	}

	return atcBuild
}
