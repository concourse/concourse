package pipelineserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
)

func (s *Server) ListPipelines(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("list-pipelines")

	pipelines, err := s.pipelinesDB.GetAllActivePipelines()
	if err != nil {
		logger.Error("failed-to-get-all-active-pipelines", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	presentedPipelines := make([]atc.Pipeline, len(pipelines))
	for i := 0; i < len(pipelines); i++ {
		pipeline := pipelines[i]

		config, _, err := s.configDB.GetConfig(pipeline.Name)
		if err != nil {
			logger.Error("call-to-get-pipeline-config-failed", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		presentedPipelines[i] = present.Pipeline(pipeline, config)
	}

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(presentedPipelines)
}
