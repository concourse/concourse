package workerserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/auth"
)

func (s *Server) ListWorkers(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("list-workers")

	teamDB := s.teamDBFactory.GetTeamDB(auth.GetAuthTeamName(r))
	savedWorkers, err := teamDB.Workers()
	if err != nil {
		logger.Error("failed-to-get-workers", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	workers := make([]atc.Worker, len(savedWorkers))
	for i, savedWorker := range savedWorkers {
		workers[i] = present.Worker(savedWorker)
	}

	json.NewEncoder(w).Encode(workers)
}
