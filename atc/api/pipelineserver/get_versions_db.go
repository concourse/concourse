package pipelineserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc/db"
)

func (s *Server) GetVersionsDB(pipelineDB db.Pipeline) http.Handler {
	logger := s.logger.Session("get-version-db")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		versionsDB, err := pipelineDB.LoadDebugVersionsDB()
		if err != nil {
			logger.Error("failed-to-get-version-db", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		err = json.NewEncoder(w).Encode(versionsDB)
		if err != nil {
			logger.Error("failed-to-encode-version-db", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
