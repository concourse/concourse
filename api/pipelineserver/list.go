package pipelineserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
)

func (s *Server) ListPipelines(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("list-pipelines")
	requestTeamName := r.FormValue(":team_name")

	authedTeamName, _, _, teamIsInAuth := auth.GetTeam(r)
	teamDB := s.teamDBFactory.GetTeamDB(authedTeamName)

	var pipelines []db.SavedPipeline
	var err error
	if teamIsInAuth && requestTeamName == authedTeamName {
		pipelines, err = teamDB.GetPipelines()
	} else {
		pipelines, err = teamDB.GetPublicPipelines()
	}

	if err != nil {
		logger.Error("failed-to-get-all-active-pipelines", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(present.Pipelines(pipelines))
}
