package resourceserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

func (s *Server) ListResources(pipelineDB db.PipelineDB) http.Handler {
	logger := s.logger.Session("list-resources")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var resources []atc.Resource

		config, _, found, err := pipelineDB.GetConfig()
		if err != nil {
			logger.Error("failed-to-get-config", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		showCheckErr := auth.IsAuthenticated(r)

		for _, resource := range config.Resources {
			dbResource, found, err := pipelineDB.GetResource(resource.Name)
			if err != nil {
				logger.Error("failed-to-get-resource", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if !found {
				logger.Debug("resource-not-found", lager.Data{"resource": resource})
				w.WriteHeader(http.StatusNotFound)
				return
			}

			resources = append(
				resources,
				present.Resource(
					resource,
					config.Groups,
					dbResource,
					showCheckErr,
				),
			)
		}

		w.WriteHeader(http.StatusOK)

		json.NewEncoder(w).Encode(resources)
	})
}
