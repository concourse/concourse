package pipelineserver

import (
	"net/http"

	"github.com/concourse/concourse/v5/atc/db"
)

func (s *Server) ExposePipeline(pipeline db.Pipeline) http.Handler {
	logger := s.logger.Session("expose-pipeline")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := pipeline.Expose()
		if err != nil {
			logger.Error("failed-to-expose-pipeline", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
