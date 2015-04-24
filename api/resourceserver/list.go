package resourceserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/db"
)

func (s *Server) ListResources(pipelineDB db.PipelineDB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var resources []atc.Resource

		config, _, err := pipelineDB.GetConfig()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		showCheckErr := s.validator.IsAuthenticated(r)

		for _, resource := range config.Resources {
			dbResource, err := pipelineDB.GetResource(resource.Name)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
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
