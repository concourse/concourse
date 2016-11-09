package workerserver

import (
	"encoding/json"
	"fmt"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	"net/http"
	"time"
)

func (s *Server) HeartbeatWorker(w http.ResponseWriter, r *http.Request) {
	var (
		registration atc.Worker
		ttl          time.Duration
		err          error
	)

	logger := s.logger.Session("heartbeat-worker")
	workerName := r.FormValue(":worker_name")

	ttlStr := r.URL.Query().Get("ttl")
	if len(ttlStr) > 0 {
		ttl, err = time.ParseDuration(ttlStr)
		if err != nil {
			logger.Error("failed-to-parse-ttl", err)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "malformed ttl")
			return
		}
	}

	err = json.NewDecoder(r.Body).Decode(&registration)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	registration.Name = workerName

	_, err = s.dbWorkerFactory.HeartbeatWorker(registration, ttl)
	if err == dbng.ErrWorkerNotPresent {
		logger.Error("failed-to-find-worker", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if err != nil {
		logger.Error("failed-to-heartbeat-worker", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
