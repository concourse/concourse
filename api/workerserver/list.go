package workerserver

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
)

func (s *Server) ListWorkers(teamDB db.TeamDB) http.Handler {
	logger := s.logger.Session("list-workers")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authTeam, authTeamFound := auth.GetTeam(r)
		if !authTeamFound {
			logger.Error("team-not-found-in-context", errors.New("team-not-found-in-context"))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		savedWorkers, err := s.dbWorkerFactory.WorkersForTeam(authTeam.Name())
		if err != nil {
			logger.Error("failed-to-get-workers", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		workers := make([]atc.Worker, len(savedWorkers))
		for i, savedWorker := range savedWorkers {
			workers[i] = present.Worker(*savedWorker)
		}

		json.NewEncoder(w).Encode(workers)
	})
}
