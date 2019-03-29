package buildserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) GetBuildArtifacts(build db.Build) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session("get-build-artifacts")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		artifacts, err := build.Artifacts()
		if err != nil {
			logger.Error("failed-to-fetch-build-artifacts", err)
			w.WriteHeader(http.StatusInternalServerError)
		}

		err = json.NewEncoder(w).Encode(present.WorkerArtifacts(artifacts))
		if err != nil {
			logger.Error("failed-to-encode-build-artifacts", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
