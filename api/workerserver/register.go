package workerserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/api/auth"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/metric"
)

type IntMetric int

func (i IntMetric) String() string {
	return strconv.Itoa(int(i))
}

func (s *Server) RegisterWorker(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("register-worker")
	var registration atc.Worker

	if !auth.IsSystem(r) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	err := json.NewDecoder(r.Body).Decode(&registration)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err = registration.Validate()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, err.Error())
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

	metric.WorkerVolumes{
		WorkerName: registration.Name,
		Volumes:    registration.ActiveVolumes,
	}.Emit(s.logger)

	var savedWorker db.Worker

	if registration.Team != "" {
		team, found, err := s.teamFactory.FindTeam(registration.Team)
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

		savedWorker, err = team.SaveWorker(registration, ttl)
		if err != nil {
			logger.Error("failed-to-save-worker", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else {
		savedWorker, err = s.dbWorkerFactory.SaveWorker(registration, ttl)
		if err != nil {
			logger.Error("failed-to-save-worker", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	gardenWorker := s.workerProvider.NewGardenWorker(logger, clock.NewClock(), savedWorker)
	err = gardenWorker.EnsureCertsVolumeExists(logger)
	if err != nil {
		logger.Error("failed-to-ensure-certs-volume", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
