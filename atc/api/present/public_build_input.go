package present

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func PublicBuildInput(input db.BuildInput, pipelineID int) atc.PublicBuildInput {
	// XXX: we probably don't need to expose this much here. name and version are all we show in the UI.
	return atc.PublicBuildInput{
		Name:            input.Name,
		Version:         atc.Version(input.Version),
		PipelineID:      pipelineID,
		FirstOccurrence: input.FirstOccurrence,
	}
}
