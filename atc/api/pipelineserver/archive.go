package pipelineserver

import (
	"net/http"

	"github.com/concourse/concourse/atc/db"
)

func (s *Server) ArchivePipeline(pipelineDB db.Pipeline) http.Handler {
	logger := s.logger.Session("archive-pipeline")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := pipelineDB.Archive()
		if err != nil {
			logger.Error("failed-to-archive-pipeline", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
