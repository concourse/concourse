package resourceserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/dbng"
)

func (s *Server) ListResources(pipeline dbng.Pipeline) http.Handler {
	logger := s.logger.Session("list-resources")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resources, err := pipeline.Resources()
		if err != nil {
			logger.Error("failed-to-get-dashboard-resources", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		showCheckErr := auth.IsAuthenticated(r)
		teamName := r.FormValue(":team_name")

		config, _, _, err := pipeline.Config()
		if err != nil {
			logger.Error("failed-to-get-pipeline-config", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var presentedResources []atc.Resource
		for _, resource := range resources {
			presentedResources = append(
				presentedResources,
				present.Resource(
					resource,
					config.Groups,
					showCheckErr,
					teamName,
				),
			)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(presentedResources)
	})
}
