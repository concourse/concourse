package resourceserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
)

func (s *Server) ListResources(pipelineDB db.PipelineDB) http.Handler {
	logger := s.logger.Session("list-resources")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dashboardResources, groupConfigs, found, err := pipelineDB.GetResources()
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

		var resources []atc.Resource
		for _, dashboardResource := range dashboardResources {
			resources = append(
				resources,
				present.Resource(
					dashboardResource.ResourceConfig,
					groupConfigs,
					dashboardResource.Resource,
					showCheckErr,
					teamName,
				),
			)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resources)
	})
}
