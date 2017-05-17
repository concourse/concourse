package resourceserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
)

func (s *Server) ListResources(_ db.PipelineDB, dbPipeline dbng.Pipeline) http.Handler {
	logger := s.logger.Session("list-resources")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resources, err := dbPipeline.Resources()
		if err != nil {
			logger.Error("failed-to-get-dashboard-resources", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		showCheckErr := auth.IsAuthenticated(r)
		teamName := r.FormValue(":team_name")

		var presentedResources []atc.Resource
		for _, resource := range resources {
			presentedResources = append(
				presentedResources,
				present.Resource(
					resource,
					dbPipeline.Config().Groups,
					showCheckErr,
					teamName,
				),
			)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(presentedResources)
	})
}
