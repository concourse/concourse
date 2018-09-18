package pipelineserver

import (
	"net/http"

	"github.com/concourse/atc/db"
)

func (s *Server) HidePipeline(pipelineDB db.Pipeline) http.Handler {
	logger := s.logger.Session("hide-pipeline")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := pipelineDB.Hide()
		if err != nil {
			logger.Error("failed-to-hide-pipeline", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
