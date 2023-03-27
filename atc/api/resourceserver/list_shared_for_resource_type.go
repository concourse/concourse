package resourceserver

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/tedsuo/rata"
)

func (s *Server) ListSharedForResourceType(pipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resourceTypeName := rata.Param(r, "resource_type_name")
		logger := s.logger.Session("list-shared-for-resource-type", lager.Data{"resource-type-name": resourceTypeName})

		resourceType, found, err := pipeline.ResourceType(resourceTypeName)
		if err != nil {
			logger.Error("failed-to-get-resource-type", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			logger.Info("resource-type-not-found")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		shared, err := resourceType.SharedResourcesAndTypes()
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
