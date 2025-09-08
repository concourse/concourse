package pipelineserver

import (
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) GetPipeline(pipeline db.Pipeline) http.Handler {
	logger := s.logger.Session("get-pipeline")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		err := sonic.ConfigDefault.NewEncoder(w).Encode(present.Pipeline(pipeline))
		if err != nil {
			logger.Error("failed-to-encode-pipeline", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
