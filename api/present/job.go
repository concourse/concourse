package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web/routes"
	"github.com/tedsuo/rata"
)

func Job(job atc.JobConfig, groups atc.GroupConfigs, finishedBuild, nextBuild *db.Build) atc.Job {
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

	inputs := make([]atc.JobInput, len(job.Inputs))
	for i, input := range job.Inputs {
		inputs[i] = atc.JobInput{
			Name:     input.Name,
			Resource: input.Resource,
			Passed:   input.Passed,
			Hidden:   input.Hidden,
			Trigger:  *input.Trigger,
		}
	}

	outputs := make([]atc.JobOutput, len(job.Outputs))
	for i, output := range job.Outputs {
		outputs[i] = atc.JobOutput{
			Resource:  output.Resource,
			PerformOn: output.PerformOn,
		}
	}

	return atc.Job{
		Name:          job.Name,
		URL:           req.URL.String(),
		FinishedBuild: presentedFinishedBuild,
		NextBuild:     presentedNextBuild,

		Inputs:  inputs,
		Outputs: outputs,

		Groups: groupNames,
	}
}
