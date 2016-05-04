package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

func BuildPreparation(preparation db.BuildPreparation) atc.BuildPreparation {
	inputs := make(map[string]atc.BuildPreparationStatus)

	for k, v := range preparation.Inputs {
		inputs[k] = atc.BuildPreparationStatus(v)
	}

	return atc.BuildPreparation{
		BuildID:             preparation.BuildID,
		PausedPipeline:      atc.BuildPreparationStatus(preparation.PausedPipeline),
		PausedJob:           atc.BuildPreparationStatus(preparation.PausedJob),
		MaxRunningBuilds:    atc.BuildPreparationStatus(preparation.MaxRunningBuilds),
		Inputs:              inputs,
		InputsSatisfied:     atc.BuildPreparationStatus(preparation.InputsSatisfied),
		MissingInputReasons: atc.MissingInputReasons(preparation.MissingInputReasons),
	}
}
