package workerserver

import "net/http"

func (s *Server) LandWorker(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("landing-worker")
	workerName := r.FormValue(":worker_name")

	worker, found, err := s.dbWorkerFactory.GetWorker(workerName)
	if err != nil {
		logger.Error("failed-finding-worker-to-land", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		logger.Error("failed-to-find-worker", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	err = worker.Land()
	if err != nil {
		logger.Error("failed-to-land-worker", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
