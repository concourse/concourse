package pipelineserver

import (
	"net/http"

	"github.com/concourse/atc/db"
)

func (s *Server) RevealPipeline(pipelineDB db.PipelineDB) http.Handler {
	logger := s.logger.Session("reveal-pipeline")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := pipelineDB.Reveal()
		if err != nil {
			logger.Error("failed-to-reveal-pipeline", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
