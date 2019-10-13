package resourceserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc/db"
)

func (s *Server) ClearResourceCache(pipeline db.Pipeline) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resourceName := r.FormValue(":resource_name")

		clearedVersions := []string{"v1", "v2", resourceName}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		json.NewEncoder(w).Encode(clearedVersions)
	})
}
