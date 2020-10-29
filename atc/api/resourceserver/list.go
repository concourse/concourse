package resourceserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) ListResources(pipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session("list-resources")

		resources, err := pipeline.Resources()
		if err != nil {
			logger.Error("failed-to-get-dashboard-resources", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		presentedResources := []atc.Resource{}
		for _, resource := range resources {
			presentedResources = append(
				presentedResources,
				present.Resource(resource),
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
