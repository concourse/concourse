package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web/routes"
	"github.com/tedsuo/rata"
)

func Job(jobName string, finishedBuild, nextBuild *db.Build) atc.Job {
	generator := rata.NewRequestGenerator("", routes.Routes)

	req, err := generator.CreateRequest(
		routes.GetJob,
		rata.Params{"job": jobName},
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

	return atc.Job{
		Name:          jobName,
		URL:           req.URL.String(),
		FinishedBuild: presentedFinishedBuild,
		NextBuild:     presentedNextBuild,
	}
}
