package infoserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
)

func (s *Server) Info(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("info")

	err := json.NewEncoder(w).Encode(atc.Info{Version: s.version, WorkerVersion: s.workerVersion})
	if err != nil {
		logger.Error("failed-to-encode-info", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
