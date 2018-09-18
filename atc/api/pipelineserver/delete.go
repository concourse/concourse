package pipelineserver

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
)

func (s *Server) DeletePipeline(pipelineDB db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session("destroying-pipeline", lager.Data{
			"name": pipelineDB.Name(),
		})

		logger.Info("start")

		err := pipelineDB.Destroy()
		if err != nil {
			logger.Error("failed", err)

			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		logger.Info("done")

		w.WriteHeader(http.StatusNoContent)
	})
}
