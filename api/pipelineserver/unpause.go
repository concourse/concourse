package pipelineserver

import (
	"net/http"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
)

func (s *Server) UnpausePipeline(_ db.PipelineDB, pipelineDB dbng.Pipeline) http.Handler {
	logger := s.logger.Session("unpause-pipeline")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := pipelineDB.Unpause()
		if err != nil {
			logger.Error("failed-to-unpause-pipeline", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
