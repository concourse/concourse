package pipelineserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/db"
)

func (s *Server) GetPipeline(pipelineDB db.PipelineDB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pipeline := pipelineDB.Pipeline()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(present.Pipeline(pipeline))
	})
}
