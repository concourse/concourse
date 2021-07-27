package present

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func Pipeline(savedPipeline db.Pipeline) atc.Pipeline {
	atcPipeline := atc.Pipeline{
		ID:            savedPipeline.ID(),
		Name:          savedPipeline.Name(),
		InstanceVars:  savedPipeline.InstanceVars(),
		TeamName:      savedPipeline.TeamName(),
		Paused:        savedPipeline.Paused(),
		PausedBy:      savedPipeline.PausedBy(),
		Public:        savedPipeline.Public(),
		Archived:      savedPipeline.Archived(),
		Groups:        savedPipeline.Groups(),
		Display:       savedPipeline.Display(),
		ParentBuildID: savedPipeline.ParentBuildID(),
		ParentJobID:   savedPipeline.ParentJobID(),
		LastUpdated:   savedPipeline.LastUpdated().Unix(),
	}

	if !savedPipeline.PausedAt().IsZero() {
		atcPipeline.PausedAt = savedPipeline.PausedAt().Unix()
	}

	return atcPipeline
}
