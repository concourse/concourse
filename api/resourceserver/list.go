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

func (s *Server) ListResources(pipelineDB db.PipelineDB, _ dbng.Pipeline) http.Handler {
	logger := s.logger.Session("list-resources")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resources, found, err := pipelineDB.GetResources()
		if err != nil {
			logger.Error("failed-to-get-dashboard-resources", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			w.WriteHeader(http.StatusNotFound)
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
					pipelineDB.Config().Groups,
					showCheckErr,
					teamName,
				),
			)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(presentedResources)
	})
}
