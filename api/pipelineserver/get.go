package pipelineserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/auth"
)

func (s *Server) GetPipeline(w http.ResponseWriter, r *http.Request) {
	pipelineName := r.FormValue(":pipeline_name")
	teamName := r.FormValue(":team_name")

	teamDB := s.teamDBFactory.GetTeamDB(teamName)
	pipeline, err := teamDB.GetPipelineByName(pipelineName)
	if err != nil {
		s.logger.Error("call-to-get-pipeline-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	authorized, response := auth.IsAuthorized(r)
	if !authorized && !pipeline.Public {
		if response == auth.Unauthorized {
			s.rejector.Unauthorized(w, r)
		} else if response == auth.Forbidden {
			s.rejector.Forbidden(w, r)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	presentedPipeline := present.Pipeline(pipeline, pipeline.Config)

	json.NewEncoder(w).Encode(presentedPipeline)
}
