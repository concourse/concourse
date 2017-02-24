package workerserver

import "net/http"

func (s *Server) DeleteWorker(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("deleting-worker")
	workerName := r.FormValue(":worker_name")

	worker, found, err := s.dbWorkerFactory.GetWorker(workerName)
	if err != nil {
		logger.Error("failed-finding-worker-to-delete", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if found {
		err := worker.Delete()
		if err != nil {
			logger.Error("failed-to-delete-worker", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}
