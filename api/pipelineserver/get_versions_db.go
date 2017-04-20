package pipelineserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
)

func (s *Server) GetVersionsDB(_ db.PipelineDB, pipelineDB dbng.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		versionsDB, _ := pipelineDB.LoadVersionsDB()
		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(versionsDB)
	})
}
