package workerserver

import (
	"io"
	"net/http"
	"strconv"
)

func (s *Server) Overloaded(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("worker-overloaded-status")
	workerName := r.FormValue(":worker_name")

	data, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Error("failed-to-read-body", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	status, err := strconv.ParseBool(string(data))
	if err != nil {
		logger.Error("failed-to-parse-boolean-from-body", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	worker, found, err := s.dbWorkerFactory.GetWorker(workerName)
	if err != nil {
		logger.Error("failed-to-find-worker", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		logger.Error("failed-to-find-worker", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	err = worker.SetOverloaded(status)
	if err != nil {
		logger.Error("failed-to-find-worker", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
