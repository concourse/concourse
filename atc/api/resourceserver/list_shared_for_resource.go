package resourceserver

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/tedsuo/rata"
)

func (s *Server) ListSharedForResource(pipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resourceName := rata.Param(r, "resource_name")
		logger := s.logger.Session("list-shared-for-resource", lager.Data{"resource-name": resourceName})

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

		shared, err := resource.SharedResourcesAndTypes()
		if err != nil {
			logger.Error("failed-to-get-shared-resource-and-types", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err = json.NewEncoder(w).Encode(shared)
		if err != nil {
			logger.Error("failed-to-encode-shared-resources-and-types", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
