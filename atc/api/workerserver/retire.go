package workerserver

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc"
)

func (s *Server) RetireWorker(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("retiring-worker")
	workerName := r.FormValue(":worker_name")

	var givenWorker atc.Worker
	err := json.NewDecoder(r.Body).Decode(&givenWorker)
	if err != nil {
		logger.Error("failed-to-decode-body", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

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

	if worker.TeamName() != givenWorker.Team {
		logger.Error("worker-belongs-to-different-team", nil, lager.Data{
			"workers_team": worker.TeamName(),
			"given_team":   givenWorker.Team,
		})
		w.WriteHeader(http.StatusBadRequest)
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
