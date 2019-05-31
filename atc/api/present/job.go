package present

import (
	"github.com/concourse/concourse/v5/atc"
	"github.com/concourse/concourse/v5/atc/db"
)

func Job(
	teamName string,
	job db.Job,
	finishedBuild db.Build,
	nextBuild db.Build,
	transitionBuild db.Build,
) atc.Job {
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
		PipelineName:         job.PipelineName(),
		TeamName:             teamName,
		DisableManualTrigger: job.Config().DisableManualTrigger,
		Paused:               job.Paused(),
		FirstLoggedBuildID:   job.FirstLoggedBuildID(),
		FinishedBuild:        presentedFinishedBuild,
		NextBuild:            presentedNextBuild,
		TransitionBuild:      presentedTransitionBuild,
		HasNewInputs:         job.HasNewInputs(),

		Inputs:  sanitizedInputs,
		Outputs: sanitizedOutputs,

		Groups: job.Tags(),
	}
}
