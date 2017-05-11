package infoserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
)

func (s *Server) Info(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(atc.Info{Version: s.version, WorkerVersion: s.workerVersion})
}
