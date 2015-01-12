package workerserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
)

func (s *Server) ListWorkers(w http.ResponseWriter, r *http.Request) {
	workerInfos, err := s.db.Workers()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	workers := make([]atc.Worker, len(workerInfos))
	for i := 0; i < len(workerInfos); i++ {
		workers[i] = present.Worker(workerInfos[i])
	}

	json.NewEncoder(w).Encode(workers)
}
