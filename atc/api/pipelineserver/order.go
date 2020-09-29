package pipelineserver

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
)

func (s *Server) OrderPipelines(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("order-pipelines")

	var pipelinesRefs atc.OrderPipelinesRequest
	if err := json.NewDecoder(r.Body).Decode(&pipelinesRefs); err != nil {
		logger.Error("invalid-json", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	teamName := r.FormValue(":team_name")
	team, found, err := s.teamFactory.FindTeam(teamName)
	if err != nil {
		logger.Error("failed-to-get-team", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		logger.Info("team-not-found")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	err = team.OrderPipelines(pipelinesRefs)
	if err != nil {
		logger.Error("failed-to-order-pipelines", err, lager.Data{
			"pipeline-refs": pipelinesRefs,
		})
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
