package pipelineserver

import (
	"encoding/json"
	"net/http"

	"github.com/pivotal-golang/lager"
)

func (s *Server) OrderPipelines(w http.ResponseWriter, r *http.Request) {
	pipelineNames := []string{}

	if err := json.NewDecoder(r.Body).Decode(&pipelineNames); err != nil {
		s.logger.Error("invalid-json", err)

		w.WriteHeader(http.StatusBadRequest)
		return
	}

	teamName := r.FormValue(":team_name")
	teamDB := s.teamDBFactory.GetTeamDB(teamName)

	err := teamDB.OrderPipelines(pipelineNames)
	if err != nil {
		s.logger.Error("failed-to-order-pipelines", err, lager.Data{
			"pipeline-names": pipelineNames,
		})

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
