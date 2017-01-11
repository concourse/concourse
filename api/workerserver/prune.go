package workerserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
)

func (s *Server) PruneWorker(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("pruning-worker")
	workerName := r.FormValue(":worker_name")

	err := s.dbWorkerFactory.PruneWorker(workerName)
	if err == dbng.ErrWorkerNotPresent {
		logger.Error("failed-to-find-worker", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if err == dbng.ErrCannotPruneRunningWorker {
		logger.Error("failed-to-prune-non-stalled-worker", err)
		responseBody := atc.PruneWorkerResponseBody{
			Stderr: "cannot prune running worker",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(responseBody)
		return
	}

	if err != nil {
		logger.Error("failed-to-prune-worker", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
