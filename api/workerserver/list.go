package workerserver

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/auth"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/db"
)

func (s *Server) ListWorkers(team db.Team) http.Handler {
	logger := s.logger.Session("list-workers")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, authTeamFound := auth.GetTeam(r)
		if !authTeamFound {
			logger.Error("team-not-found-in-context", errors.New("team-not-found-in-context"))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		savedWorkers, err := team.Workers()
		if err != nil {
			logger.Error("failed-to-get-workers", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		workers := make([]atc.Worker, len(savedWorkers))
		for i, savedWorker := range savedWorkers {
			workers[i] = present.Worker(savedWorker)
		}

		err = json.NewEncoder(w).Encode(workers)
		if err != nil {
			logger.Error("failed-to-encode-workers", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
