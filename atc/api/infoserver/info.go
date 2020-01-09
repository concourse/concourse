package infoserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc"
)

func (s *Server) Info(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("info")

	message, err := s.wall.GetMessage()
	if err != nil {
		logger.Error("failed-to-get-wall-message", err)
		w.WriteHeader(http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(atc.Info{Version: s.version,
		WorkerVersion: s.workerVersion,
		ExternalURL:   s.externalURL,
		ClusterName:   s.clusterName,
		WallMessage:   message})
	if err != nil {
		logger.Error("failed-to-encode-info", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
