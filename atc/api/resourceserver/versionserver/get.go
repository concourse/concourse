package versionserver

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/concourse/concourse/v5/atc/db"
)

// IMPORTANT: This is not yet tested because it is not yet used
func (s *Server) GetResourceVersion(pipeline db.Pipeline) http.Handler {
	logger := s.logger.Session("get-resource-version")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		versionID, err := strconv.Atoi(r.FormValue(":resource_config_version_id"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		version, found, err := pipeline.ResourceVersion(versionID)
		if err != nil {
			logger.Error("failed-to-get-resource-version", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		w.WriteHeader(http.StatusOK)

		err = json.NewEncoder(w).Encode(version)
		if err != nil {
			logger.Error("failed-to-encode-resource-version", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
