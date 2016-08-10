package pipelineserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
)

func (s *Server) GetPipeline(w http.ResponseWriter, r *http.Request) {
	pipelineName := r.FormValue(":pipeline_name")

	requestTeamName := r.FormValue(":team_name")
	authedTeamName, _, _, teamIsInAuth := auth.GetTeam(r)

	var pipeline db.SavedPipeline
	var err error
	var found bool

	teamDB := s.teamDBFactory.GetTeamDB(requestTeamName)

	if teamIsInAuth && requestTeamName == authedTeamName {
		pipeline, found, err = teamDB.GetPipelineByName(pipelineName)
	} else {
		pipeline, found, err = teamDB.GetPublicPipelineByName(pipelineName)
	}

	if err != nil {
		s.logger.Error("call-to-get-pipeline-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		s.logger.Error("pipeline-not-found", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(present.Pipeline(pipeline, pipeline.Config))
}
