package workerserver

import (
	"net/http"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/v8/atc/api/accessor"
)

func (s *Server) DeleteWorker(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("deleting-worker")
	workerName := r.FormValue(":worker_name")
	acc := accessor.GetAccessor(r)

	givenTeam := r.URL.Query().Get("team_name")

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

	if teamName != givenTeam {
		logger.Error("worker-belongs-to-different-team", nil, lager.Data{
			"workers_team": teamName,
			"given_team":   givenTeam,
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
