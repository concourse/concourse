package workerserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/metric"
)

type IntMetric int

func (i IntMetric) String() string {
	return strconv.Itoa(int(i))
}

func (s *Server) RegisterWorker(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("register-worker")
	var registration atc.Worker

	isSystem, present := r.Context().Value("system").(bool)

	if !present || !isSystem {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	err := json.NewDecoder(r.Body).Decode(&registration)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if len(registration.GardenAddr) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "missing address")
		return
	}

	var ttl time.Duration

	ttlStr := r.URL.Query().Get("ttl")
	if len(ttlStr) > 0 {
		ttl, err = time.ParseDuration(ttlStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "malformed ttl")
			return
		}
	}

	if registration.Name == "" {
		registration.Name = registration.GardenAddr
	}

	metric.WorkerContainers{
		WorkerName: registration.Name,
		Containers: registration.ActiveContainers,
	}.Emit(s.logger)

	if registration.Team != "" {
		team, found, err := s.dbTeamFactory.FindTeam(registration.Team)
		if err != nil {
			logger.Error("failed-to-get-team", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			logger.Error("team-not-found", errors.New("team-not-found"), lager.Data{"team-name": registration.Team})
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		_, err = team.SaveWorker(registration, ttl)
		if err != nil {
			logger.Error("failed-to-save-worker", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else {
		_, err := s.dbWorkerFactory.SaveWorker(registration, ttl)
		if err != nil {
			logger.Error("failed-to-save-worker", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}
