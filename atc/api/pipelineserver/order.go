package pipelineserver

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"
)

func (s *Server) OrderPipelines(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("order-pipelines")

	var pipelinesNames []string
	if err := json.NewDecoder(r.Body).Decode(&pipelinesNames); err != nil {
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

	err = team.OrderPipelines(pipelinesNames)
	if err != nil {
		logger.Error("failed-to-order-pipelines", err, lager.Data{
			"pipeline_names": pipelinesNames,
		})
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
