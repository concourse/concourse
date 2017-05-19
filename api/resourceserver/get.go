package resourceserver

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/dbng"
)

func (s *Server) GetResource(dbPipeline dbng.Pipeline) http.Handler {
	logger := s.logger.Session("get-resource")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resourceName := r.FormValue(":resource_name")
		teamName := r.FormValue(":team_name")

		dbResource, found, err := dbPipeline.Resource(resourceName)
		if err != nil {
			logger.Error("failed-to-get-resource", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			logger.Debug("resource-not-found", lager.Data{"resource": resourceName})
			w.WriteHeader(http.StatusNotFound)
			return
		}

		resource := present.Resource(
			dbResource,
			dbPipeline.Config().Groups,
			auth.IsAuthenticated(r),
			teamName,
		)

		w.WriteHeader(http.StatusOK)

		json.NewEncoder(w).Encode(resource)
	})
}
