package resourceserver

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) UnpinResource(pipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resourceName := r.FormValue(":resource_name")

		logger := s.logger.Session("unpin-resource-version", lager.Data{
			"resource": resourceName,
		})

		resource, found, err := pipeline.Resource(resourceName)
		if err != nil {
			logger.Error("failed-to-get-resource", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !found {
			logger.Info("resource-not-found")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		err = resource.UnpinVersion()
		if err != nil {
			logger.Error("failed-to-unpin-resource-version", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
