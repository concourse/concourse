package present

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
)

func Pipelines(savedPipelines []dbng.Pipeline) []atc.Pipeline {
	pipelines := make([]atc.Pipeline, len(savedPipelines))

	for i := range savedPipelines {
		pipelines[i] = Pipeline(savedPipelines[i])
	}

	return pipelines
}
