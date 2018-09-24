package present

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func Pipeline(savedPipeline db.Pipeline) atc.Pipeline {
	return atc.Pipeline{
		ID:       savedPipeline.ID(),
		Name:     savedPipeline.Name(),
		TeamName: savedPipeline.TeamName(),
		Paused:   savedPipeline.Paused(),
		Archived: savedPipeline.Archived(),
		Public:   savedPipeline.Public(),
		Groups:   savedPipeline.Groups(),
	}
}
