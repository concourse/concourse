package versionserver

import (
	"encoding/json"
	"net/http"
	"strconv"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) GetResourceVersion(pipeline db.Pipeline) http.Handler {
	logger := s.logger.Session("get-resource-version")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		versionID, err := strconv.Atoi(r.FormValue(":resource_config_version_id"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		resourceName := r.FormValue(":resource_name")
		resource, found, err := pipeline.Resource(resourceName)
		if err != nil {
			logger.Error("failed-to-get-resource", err, lager.Data{"resource-name": resourceName})
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !found {
			logger.Info("resource-not-found", lager.Data{"resource-name": resourceName})
			w.WriteHeader(http.StatusNotFound)
			return
		}

		version, found, err := pipeline.ResourceVersion(resource.ID(), versionID)
		if err != nil {
			logger.Error("failed-to-get-resource-version", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		teamName := r.FormValue(":team_name")
		acc := accessor.GetAccessor(r)
		hideMetadata := !resource.Public() && !acc.IsAuthorized(teamName)

		version = present.ResourceVersion(hideMetadata, version)

		w.Header().Set("Content-Type", "application/json")

		w.WriteHeader(http.StatusOK)

		err = json.NewEncoder(w).Encode(version)
		if err != nil {
			logger.Error("failed-to-encode-resource-version", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
