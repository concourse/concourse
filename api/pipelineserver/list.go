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
	teamDB := s.teamDBFactory.GetTeamDB(requestTeamName)

	var pipelines []db.SavedPipeline
	var err error

	authTeam, authTeamFound := auth.GetTeam(r)
	if authTeamFound && authTeam.IsAuthorized(requestTeamName) {
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
