package present

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func PublicBuildInput(input db.BuildInput, pipelineID int) atc.PublicBuildInput {
	return atc.PublicBuildInput{
		Name:                input.Name,
		ResourceDisplayName: input.ResourceDisplayName,
		Version:             atc.Version(input.Version),
		PipelineID:          pipelineID,
		FirstOccurrence:     input.FirstOccurrence,
	}
}

func PublicBuildOutput(output db.BuildOutput) atc.PublicBuildOutput {
	return atc.PublicBuildOutput{
		Name:                output.Name,
		ResourceDisplayName: output.ResourceDisplayName,
		Version:             atc.Version(output.Version),
	}
}
