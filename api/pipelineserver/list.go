package pipelineserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
)

func (s *Server) ListPipelines(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("list-pipelines")
	pipelines, err := s.db.GetAllActivePipelines()
	if err != nil {
		logger.Error("failed-to-get-all-active-pipelines", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	presentedPipelines := make([]atc.Pipeline, len(pipelines))
	for i := 0; i < len(pipelines); i++ {
		presentedPipelines[i] = present.Pipeline(pipelines[i])
	}

	json.NewEncoder(w).Encode(presentedPipelines)
}
