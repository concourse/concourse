package versionserver

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/concourse/atc/db"
)

func (s *Server) GetCausality(pipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		versionIDString := r.FormValue(":resource_version_id")
		versionID, _ := strconv.Atoi(versionIDString)

		causality, err := pipeline.Causality(versionID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		w.WriteHeader(http.StatusOK)

		_ = json.NewEncoder(w).Encode(causality)
	})
}
