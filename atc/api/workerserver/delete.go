package workerserver

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
)

func (s *Server) DeleteWorker(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("deleting-worker")
	workerName := r.FormValue(":worker_name")
	acc := accessor.GetAccessor(r)

	var givenWorker atc.Worker
	err := json.NewDecoder(r.Body).Decode(&givenWorker)
	if err != nil {
		logger.Error("failed-to-decode-body", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	worker, found, err := s.dbWorkerFactory.GetWorker(workerName)
	if err != nil || !found {
		logger.Error("failed-finding-worker-to-delete", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	teamName := worker.TeamName()
	var teamAuthorized bool
	if teamName != "" {
		teamAuthorized = acc.IsAuthorized(teamName)
	}

	if teamName != givenWorker.Team {
		logger.Error("worker-belongs-to-different-team", nil, lager.Data{
			"workers_team": teamName,
			"given_team":   givenWorker.Team,
		})
		w.WriteHeader(http.StatusBadRequest)
		return
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
