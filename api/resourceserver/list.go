package resourceserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/accessor"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/db"
)

func (s *Server) ListResources(pipeline db.Pipeline) http.Handler {
	logger := s.logger.Session("list-resources")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resources, err := pipeline.Resources()
		if err != nil {
			logger.Error("failed-to-get-dashboard-resources", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		acc := accessor.GetAccessor(r)
		showCheckErr := acc.IsAuthenticated()
		teamName := r.FormValue(":team_name")

		var presentedResources []atc.Resource
		for _, resource := range resources {
			presentedResources = append(
				presentedResources,
				present.Resource(
					resource,
					pipeline.Groups(),
					showCheckErr,
					teamName,
				),
			)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err = json.NewEncoder(w).Encode(presentedResources)
		if err != nil {
			logger.Error("failed-to-encode-resources", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
