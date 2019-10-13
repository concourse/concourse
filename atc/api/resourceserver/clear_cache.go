package resourceserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc/db"
)

func (s *Server) ClearResourceCaches(pipeline db.Pipeline) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resourceName := r.FormValue(":resource_name")

		clearedVersions := []string{resourceName, "v1"}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		json.NewEncoder(w).Encode(clearedVersions)
	})
}

func (s *Server) ClearResourceCache(pipeline db.Pipeline) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//resourceName := r.FormValue(":resource_name")
		resourceConfigVersionID := r.FormValue(":resource_config_version_id")

		clearedVersions := []string{resourceConfigVersionID}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		json.NewEncoder(w).Encode(clearedVersions)
	})
}
