package pipelineserver

import (
	"net/http"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
)

func (s *Server) ExposePipeline(_ db.PipelineDB, pipelineDB dbng.Pipeline) http.Handler {
	logger := s.logger.Session("expose-pipeline")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := pipelineDB.Expose()
		if err != nil {
			logger.Error("failed-to-expose-pipeline", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
