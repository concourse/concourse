package pipelineserver

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/db"
)

func (s *Server) getPipelines(teamName string, all bool) ([]atc.Pipeline, error) {
	teamDB := s.teamDBFactory.GetTeamDB(teamName)
	var pipelines []db.SavedPipeline
	var err error

	if all {
		if teamName == "" {
			pipelines, err = s.pipelinesDB.GetAllPublicPipelines()
		} else {
			pipelines, err = teamDB.GetAllPipelines()
		}
	} else {
		pipelines, err = teamDB.GetPipelines()
	}

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
