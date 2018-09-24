package pipelineserver

import (
	"net/http"

	"github.com/concourse/concourse/atc/db"
)

func (s *Server) UnarchivePipeline(pipelineDB db.Pipeline) http.Handler {
	logger := s.logger.Session("unarchive-pipeline")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := pipelineDB.Unarchive()
		if err != nil {
			logger.Error("failed-to-unarchive-pipeline", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
