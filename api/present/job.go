package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web/routes"
	"github.com/tedsuo/rata"
)

func Job(dbJob db.Job, job atc.JobConfig, groups atc.GroupConfigs, finishedBuild, nextBuild *db.Build) atc.Job {
	generator := rata.NewRequestGenerator("", routes.Routes)

	req, err := generator.CreateRequest(
		routes.GetJob,
		rata.Params{"job": job.Name},
		nil,
	)
	if err != nil {
		panic("failed to generate url: " + err.Error())
	}

	var presentedNextBuild, presentedFinishedBuild *atc.Build

	if nextBuild != nil {
		presented := Build(*nextBuild)
		presentedNextBuild = &presented
	}

	if finishedBuild != nil {
		presented := Build(*finishedBuild)
		presentedFinishedBuild = &presented
	}

	groupNames := []string{}
	for _, group := range groups {
		for _, name := range group.Jobs {
			if name == job.Name {
				groupNames = append(groupNames, group.Name)
			}
		}
	}

	return atc.Job{
		Name:          job.Name,
		URL:           req.URL.String(),
		Paused:        dbJob.Paused,
		FinishedBuild: presentedFinishedBuild,
		NextBuild:     presentedNextBuild,

		Inputs:  job.Inputs(),
		Outputs: job.Outputs(),

		Groups: groupNames,
	}
}
