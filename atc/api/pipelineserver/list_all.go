package pipelineserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
)

// show all public pipelines and team private pipelines if authorized
func (s *Server) ListAllPipelines(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("list-all-pipelines")

	acc := accessor.GetAccessor(r)

	var pipelines []db.Pipeline
	var err error

	if acc.IsAdmin() {
		pipelines, err = s.pipelineFactory.AllPipelines()
	} else {
		pipelines, err = s.pipelineFactory.VisiblePipelines(acc.TeamNames())
	}

	if err != nil {
		logger.Error("failed-to-get-all-visible-pipelines", err)
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
