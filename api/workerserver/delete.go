package workerserver

import "net/http"

func (s *Server) DeleteWorker(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("deleting-worker")
	workerName := r.FormValue(":worker_name")

	err := s.dbWorkerFactory.DeleteWorker(workerName)
	if err != nil {
		logger.Error("failed-to-delete-worker", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
