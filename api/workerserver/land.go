package workerserver

import (
	"net/http"

	"github.com/concourse/atc/dbng"
)

func (s *Server) LandWorker(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("landing-worker")
	workerName := r.FormValue(":worker_name")

	_, err := s.dbWorkerFactory.LandWorker(workerName)
	if err == dbng.ErrWorkerNotPresent {
		logger.Error("failed-to-find-worker", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if err != nil {
		logger.Error("failed-to-land-worker", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
