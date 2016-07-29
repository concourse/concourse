package pipelineserver

import (
	"net/http"

	"github.com/concourse/atc/db"
)

func (s *Server) ConcealPipeline(pipelineDB db.PipelineDB) http.Handler {
	logger := s.logger.Session("conceal-pipeline")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := pipelineDB.Conceal()
		if err != nil {
			logger.Error("failed-to-conceal-pipeline", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
