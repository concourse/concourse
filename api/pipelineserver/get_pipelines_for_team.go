package pipelineserver

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
)

func (s *Server) getPipelinesForTeam(teamName string) ([]atc.Pipeline, error) {
	teamDB := s.teamDBFactory.GetTeamDB(teamName)
	pipelines, err := teamDB.GetPipelines()
	if err != nil {
		return []atc.Pipeline{}, err
	}

	presentedPipelines := make([]atc.Pipeline, len(pipelines))
	for i := 0; i < len(pipelines); i++ {
		pipeline := pipelines[i]
		presentedPipelines[i] = present.Pipeline(pipeline, pipeline.Config)
	}

	return presentedPipelines, nil
}
