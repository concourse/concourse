package pipelineserver

import (
	"net/http"

	"github.com/concourse/concourse/atc/db"
)

func (s *Server) ArchivePipeline(pipelineDB db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.enableArchivePipeline {
			http.Error(w, "endpoint is not enabled", http.StatusForbidden)
			return
		}
		s.logger.Debug("archive-pipeline")
		err := pipelineDB.Archive()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			s.logger.Error("archive-pipeline", err)
		}
	})
}
