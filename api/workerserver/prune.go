package workerserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

func (s *Server) PruneWorker(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("pruning-worker")
	workerName := r.FormValue(":worker_name")

	worker, found, err := s.dbWorkerFactory.GetWorker(workerName)
	if err != nil {
		logger.Error("failed-finding-worker-to-prune", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		logger.Error("failed-to-find-worker", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	err = worker.Prune()
	if err == db.ErrWorkerNotPresent {
		logger.Error("failed-to-find-worker", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if err == db.ErrCannotPruneRunningWorker {
		logger.Error("failed-to-prune-non-stalled-worker", err)
		responseBody := atc.PruneWorkerResponseBody{
			Stderr: "cannot prune running worker",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		err = json.NewEncoder(w).Encode(responseBody)
		if err != nil {
			logger.Error("failed-to-encode-prune-worker-response-body", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	if err != nil {
		logger.Error("failed-to-prune-worker", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
