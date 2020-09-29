package present

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func Job(
	teamName string,
	job db.Job,
	inputs []atc.JobInput,
	outputs []atc.JobOutput,
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
	for _, input := range inputs {
		sanitizedInputs = append(sanitizedInputs, atc.JobInput{
			Name:     input.Name,
			Resource: input.Resource,
			Passed:   input.Passed,
			Trigger:  input.Trigger,
		})
	}

	sanitizedOutputs := []atc.JobOutput{}
	for _, output := range outputs {
		sanitizedOutputs = append(sanitizedOutputs, atc.JobOutput{
			Name:     output.Name,
			Resource: output.Resource,
		})
	}

	return atc.Job{
		ID: job.ID(),

		Name:                 job.Name(),
		PipelineID:           job.PipelineID(),
		PipelineName:         job.PipelineName(),
		PipelineInstanceVars: job.PipelineInstanceVars(),
		TeamName:             teamName,
		DisableManualTrigger: job.DisableManualTrigger(),
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
