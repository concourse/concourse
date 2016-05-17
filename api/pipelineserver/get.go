package pipelineserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc/api/present"
)

func (s *Server) GetPipeline(w http.ResponseWriter, r *http.Request) {
	pipelineName := r.FormValue(":pipeline_name")
	teamName := r.FormValue(":team_name")

	pipeline, err := s.pipelinesDB.GetPipelineByTeamNameAndName(teamName, pipelineName)
	if err != nil {
		s.logger.Error("call-to-get-pipeline-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	config, _, _, err := s.configDB.GetConfig(teamName, pipelineName)
	if err != nil {
		s.logger.Error("call-to-get-pipeline-config-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	presentedPipeline := present.Pipeline(teamName, pipeline, config)

	json.NewEncoder(w).Encode(presentedPipeline)
}
