package pipelineserver

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
)

func (s *Server) DeletePipeline(_ db.PipelineDB, pipeline dbng.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session("destroying-pipeline", lager.Data{
			"name": pipeline.Name(),
		})

		logger.Info("start")

		err := pipeline.Destroy()
		if err != nil {
			s.logger.Error("failed", err)

			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		logger.Info("done")

		w.WriteHeader(http.StatusNoContent)
	})
}
