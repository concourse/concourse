package pipelineserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc/api/auth"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/db"
)

func (s *Server) ListPipelines(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("list-pipelines")
	requestTeamName := r.FormValue(":team_name")
	team, found, err := s.teamFactory.FindTeam(requestTeamName)
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

	var pipelines []db.Pipeline

	authTeam, authTeamFound := auth.GetTeam(r)
	if authTeamFound && authTeam.IsAuthorized(requestTeamName) {
		pipelines, err = team.Pipelines()
	} else {
		pipelines, err = team.PublicPipelines()
	}

	if err != nil {
		logger.Error("failed-to-get-all-active-pipelines", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(present.Pipelines(pipelines))
	if err != nil {
		logger.Error("failed-to-encode-pipelines", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
