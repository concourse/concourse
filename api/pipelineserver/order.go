package pipelineserver

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"
)

func (s *Server) OrderPipelines(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("order-pipelines")

	var pipelineNames []string
	if err := json.NewDecoder(r.Body).Decode(&pipelineNames); err != nil {
		logger.Error("invalid-json", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	teamName := r.FormValue(":team_name")
	teamDB := s.teamDBFactory.GetTeamDB(teamName)

	err := teamDB.OrderPipelines(pipelineNames)
	if err != nil {
		logger.Error("failed-to-order-pipelines", err, lager.Data{
			"pipeline-names": pipelineNames,
		})
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
