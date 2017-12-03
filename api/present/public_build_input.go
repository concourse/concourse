package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

func PublicBuildInput(input db.BuildInput, pipelineID int) atc.PublicBuildInput {
	metadata := make([]atc.MetadataField, 0, len(input.Metadata))
	for _, meta := range input.Metadata {
		metadata = append(metadata, atc.MetadataField(meta))
	}

	return atc.PublicBuildInput{
		Name:            input.Name,
		Resource:        input.Resource,
		Type:            input.Type,
		Version:         atc.Version(input.Version),
		Metadata:        metadata,
		PipelineID:      pipelineID,
		FirstOccurrence: input.FirstOccurrence,
	}
}
