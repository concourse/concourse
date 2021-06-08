package versionserver

import (
	"encoding/json"
	"net/http"
	"strconv"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc/db"
)

func (s *Server) GetUpstreamResourceCausality(pipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session("upstream-causality")

		versionIDString := r.FormValue(":resource_config_version_id")
		resourceName := r.FormValue(":resource_name")
		versionID, _ := strconv.Atoi(versionIDString)

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

		causality, found, err := resource.UpstreamCausality(versionID)
		if err != nil {
			logger.Error("failed-to-fetch", err, lager.Data{"resource-name": resourceName, "resource-config-version": versionID})
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			logger.Info("resource-version-not-found", lager.Data{"resource-name": resourceName, "resource-config-version": versionID})
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(causality)
		if err != nil {
			logger.Error("failed-to-encode", err, lager.Data{"resource-name": resourceName, "resource-config-version": versionID})
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}
