package resourceserver

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) GetResource(pipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resourceName := r.FormValue(":resource_name")

		logger := s.logger.Session("get-resource", lager.Data{
			"resource": resourceName,
		})

		dbResource, found, err := pipeline.Resource(resourceName)
		if err != nil {
			logger.Error("failed-to-get-resource", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			logger.Info("resource-not-found")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		resource := present.Resource(dbResource)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		err = json.NewEncoder(w).Encode(resource)
		if err != nil {
			logger.Error("failed-to-encode-resource", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
