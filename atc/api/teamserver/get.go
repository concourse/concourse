package teamserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/v8/atc/api/present"
	"github.com/concourse/concourse/v8/atc/db"
)

func (s *Server) GetTeam(team db.Team) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session("get-team")

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(present.Team(team)); err != nil {
			logger.Error("failed-to-encode-team", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})
}
