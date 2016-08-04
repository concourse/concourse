package resourceserver

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/tedsuo/rata"
)

func (s *Server) PauseResource(pipelineDB db.PipelineDB) http.Handler {
	logger := s.logger.Session("pause-resource")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resourceName := rata.Param(r, "resource_name")

		_, found, err := pipelineDB.GetResource(resourceName)
		if err != nil {
			logger.Error("failed-to-get-resource", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			logger.Debug("resource-not-found", lager.Data{"resource": resourceName})
			w.WriteHeader(http.StatusNotFound)
			return
		}

		err = pipelineDB.PauseResource(resourceName)
		if err != nil {
			logger.Error("failed-to-pause-resource", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
