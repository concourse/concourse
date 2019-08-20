package workerserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) ListWorkers(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("list-workers")

	var (
		workers []db.Worker
		err     error
	)

	acc := accessor.GetAccessor(r)

	if acc.IsAdmin() {
		workers, err = s.dbWorkerFactory.Workers()
	} else {
		workers, err = s.dbWorkerFactory.VisibleWorkers(acc.TeamNames())
	}

	if err != nil {
		logger.Error("failed-to-get-workers", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	atcWorkers := make([]atc.Worker, len(workers))
	for i, savedWorker := range workers {
		atcWorkers[i] = present.Worker(savedWorker)
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(atcWorkers)
	if err != nil {
		logger.Error("failed-to-encode-workers", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
