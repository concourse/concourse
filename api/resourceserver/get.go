package resourceserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
)

func (s *Server) GetResource(pipelineDB db.PipelineDB) http.Handler {
	logger := s.logger.Session("get-resource")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		config, _, found, err := pipelineDB.GetConfig()
		if err != nil {
			logger.Error("failed-to-get-config", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			logger.Info("config-not-found")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		resourceName := r.FormValue(":resource_name")

		resourceConfig, resourceFound := config.Resources.Lookup(resourceName)
		if !resourceFound {
			logger.Info("resource-not-in-config")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		dbResource, err := pipelineDB.GetResource(resourceName)
		if err != nil {
			logger.Error("failed-to-get-resource", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		resource := present.Resource(
			resourceConfig,
			config.Groups,
			dbResource,
			auth.IsAuthenticated(r),
		)

		w.WriteHeader(http.StatusOK)

		json.NewEncoder(w).Encode(resource)
	})
}
