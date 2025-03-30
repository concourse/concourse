package pipelineserver

import (
	"net/http"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) DeletePipeline(pipelineDB db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session("destroying-pipeline", lager.Data{
			"name": pipelineDB.Name(),
		})

		logger.Debug("start")

		err := pipelineDB.Destroy()
		if err != nil {
			logger.Error("failed", err)

			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		logger.Debug("done")

		w.WriteHeader(http.StatusNoContent)
	})
}
