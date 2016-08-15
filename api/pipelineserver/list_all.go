package pipelineserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
)

// show all public pipelines and team private pipelines if authorized
func (s *Server) ListAllPipelines(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("list-all-pipelines")
	authTeam, authTeamFound := auth.GetTeam(r)

	var pipelines []db.SavedPipeline
	var err error
	if authTeamFound {
		pipelines, err = s.teamDBFactory.GetTeamDB(authTeam.Name()).GetPrivateAndAllPublicPipelines()
	} else {
		pipelines, err = s.pipelinesDB.GetAllPublicPipelines()
	}

	if err != nil {
		logger.Error("failed-to-get-all-active-pipelines", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(present.Pipelines(pipelines))
}
