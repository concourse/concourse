package pipelineserver

import (
	"net/http"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
)

func (s *Server) PausePipeline(pipelineDB db.PipelineDB, _ dbng.Pipeline) http.Handler {
	logger := s.logger.Session("pause-pipeline")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := pipelineDB.Pause()
		if err != nil {
			logger.Error("failed-to-pause-pipeline", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
