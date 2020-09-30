package workerserver

import (
	"net/http"

	"github.com/concourse/concourse/atc"
)

func (s *Server) RetireWorker(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("retiring-worker")
	workerName := atc.GetParam(r, ":worker_name")

	worker, found, err := s.dbWorkerFactory.GetWorker(workerName)

	if err != nil {
		logger.Error("failed-finding-worker-to-retire", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		logger.Error("failed-to-find-worker", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	err = worker.Retire()

	if err != nil {
		logger.Error("failed-to-retire-worker", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
