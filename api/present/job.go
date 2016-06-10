package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web"
	"github.com/tedsuo/rata"
)

func Job(
	teamName string,
	dbJob db.SavedJob,
	job atc.JobConfig,
	groups atc.GroupConfigs,
	finishedBuild db.Build,
	nextBuild db.Build,
) atc.Job {
	generator := rata.NewRequestGenerator("", web.Routes)

	req, err := generator.CreateRequest(
		web.GetJob,
		rata.Params{
			"job":           job.Name,
			"pipeline_name": dbJob.PipelineName,
			"team_name":     teamName,
		},
		nil,
	)
	if err != nil {
		panic("failed to generate url: " + err.Error())
	}

	var presentedNextBuild, presentedFinishedBuild *atc.Build

	if nextBuild != nil {
		presented := Build(nextBuild)
		presentedNextBuild = &presented
	}

	if finishedBuild != nil {
		presented := Build(finishedBuild)
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

	sanitizedInputs := []atc.JobInput{}
	for _, input := range config.JobInputs(job) {
		sanitizedInputs = append(sanitizedInputs, atc.JobInput{
			Name:     input.Name,
			Resource: input.Resource,
			Passed:   input.Passed,
			Trigger:  input.Trigger,
		})
	}

	sanitizedOutputs := []atc.JobOutput{}
	for _, output := range config.JobOutputs(job) {
		sanitizedOutputs = append(sanitizedOutputs, atc.JobOutput{
			Name:     output.Name,
			Resource: output.Resource,
		})
	}

	return atc.Job{
		Name:                 job.Name,
		URL:                  req.URL.String(),
		DisableManualTrigger: job.DisableManualTrigger,
		Paused:               dbJob.Paused,
		FirstLoggedBuildID:   dbJob.FirstLoggedBuildID,
		FinishedBuild:        presentedFinishedBuild,
		NextBuild:            presentedNextBuild,

		Inputs:  sanitizedInputs,
		Outputs: sanitizedOutputs,

		Groups: groupNames,
	}
}
