package workerserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
)

func (s *Server) ListWorkers(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("list-workers")
	savedWorkers, err := s.db.Workers()
	if err != nil {
		logger.Error("failed-to-get-workers", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	workers := make([]atc.Worker, len(savedWorkers))
	for i, savedWorker := range savedWorkers {
		workers[i] = present.Worker(savedWorker.WorkerInfo)
	}

	json.NewEncoder(w).Encode(workers)
}
