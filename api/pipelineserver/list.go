package pipelineserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
)

func (s *Server) ListPipelines(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("list-pipelines")
	teamName := r.FormValue(":team_name")

	pipelines, err := s.getPipelines(teamName, false)
	if err != nil {
		logger.Error("failed-to-get-all-active-pipelines", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	authorized, _ := auth.IsAuthorized(r)
	if !authorized {
		publicPipelines := []atc.Pipeline{}
		for _, pipeline := range pipelines {
			if pipeline.Public {
				publicPipelines = append(publicPipelines, pipeline)
			}
		}
		pipelines = publicPipelines
	}

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(pipelines)
}
