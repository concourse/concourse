package buildserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/v5/atc/api/present"
	"github.com/concourse/concourse/v5/atc/db"
)

func (s *Server) GetBuild(build db.Build) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session("get-build")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		err := json.NewEncoder(w).Encode(present.Build(build))
		if err != nil {
			logger.Error("failed-to-encode-build", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
