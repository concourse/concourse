package workerserver

import (
	"net/http"

	"github.com/concourse/atc/api/accessor"
)

func (s *Server) DeleteWorker(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("deleting-worker")

	workerName := r.FormValue(":worker_name")
	acc := accessor.GetAccessor(r)

	worker, found, err := s.dbWorkerFactory.GetWorker(workerName)
	if err != nil {
		logger.Error("failed-finding-worker-to-delete", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	teamName := worker.TeamName()
	var teamAuthorized bool
	if teamName != "" {
		teamAuthorized = acc.IsAuthorized(teamName)
	}

	if found && (acc.IsAdmin() || acc.IsSystem() || teamAuthorized) {
		err := worker.Delete()
		if err != nil {
			logger.Error("failed-to-delete-worker", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}
