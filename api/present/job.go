package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web"
	"github.com/tedsuo/rata"
)

func Job(
	teamName string,
	job db.Job,
	groups atc.GroupConfigs,
	finishedBuild db.Build,
	nextBuild db.Build,
	transitionBuild db.Build,
) atc.Job {
	generator := rata.NewRequestGenerator("", web.Routes)

	req, err := generator.CreateRequest(
		web.GetJob,
		rata.Params{
			"job":           job.Name(),
			"pipeline_name": job.PipelineName(),
			"team_name":     teamName,
		},
		nil,
	)
	if err != nil {
		panic("failed to generate url: " + err.Error())
	}

	var presentedNextBuild, presentedFinishedBuild, presentedTransitionBuild *atc.Build

	if nextBuild != nil {
		presented := Build(nextBuild)
		presentedNextBuild = &presented
	}

	if finishedBuild != nil {
		presented := Build(finishedBuild)
		presentedFinishedBuild = &presented
	}

	if transitionBuild != nil {
		presented := Build(transitionBuild)
		presentedTransitionBuild = &presented
	}

	groupNames := []string{}
	for _, group := range groups {
		for _, name := range group.Jobs {
			if name == job.Name() {
				groupNames = append(groupNames, group.Name)
			}
		}
	}

	sanitizedInputs := []atc.JobInput{}
	for _, input := range job.Config().Inputs() {
		sanitizedInputs = append(sanitizedInputs, atc.JobInput{
			Name:     input.Name,
			Resource: input.Resource,
			Passed:   input.Passed,
			Trigger:  input.Trigger,
		})
	}

	sanitizedOutputs := []atc.JobOutput{}
	for _, output := range job.Config().Outputs() {
		sanitizedOutputs = append(sanitizedOutputs, atc.JobOutput{
			Name:     output.Name,
			Resource: output.Resource,
		})
	}

	return atc.Job{
		ID: job.ID(),

		Name:                 job.Name(),
		URL:                  req.URL.String(),
		DisableManualTrigger: job.Config().DisableManualTrigger,
		Paused:               job.Paused(),
		FirstLoggedBuildID:   job.FirstLoggedBuildID(),
		FinishedBuild:        presentedFinishedBuild,
		NextBuild:            presentedNextBuild,
		TransitionBuild:      presentedTransitionBuild,

		Inputs:  sanitizedInputs,
		Outputs: sanitizedOutputs,

		Groups: groupNames,
	}
}
