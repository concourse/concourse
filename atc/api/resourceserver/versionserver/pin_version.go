package versionserver

import (
	"net/http"
	"strconv"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) PinResourceVersion(pipeline db.Pipeline) http.Handler {
	logger := s.logger.Session("pin-resource-version")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resourceName := r.FormValue(":resource_name")
		resource, found, err := pipeline.Resource(resourceName)
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

		resourceConfigVersionID, err := strconv.Atoi(r.FormValue(":resource_config_version_id"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		found, err = resource.PinVersion(resourceConfigVersionID)
		if err != nil {
			logger.Error("failed-to-pin-resource-version", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !found {
			logger.Debug("resource-version-id-not-found", lager.Data{"resource_config_version_id": resourceConfigVersionID})
			w.WriteHeader(http.StatusNotFound)
		}
	})
}
