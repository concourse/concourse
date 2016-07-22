package teamserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/metric"
	"github.com/pivotal-golang/lager"
)

type IntMetric int

func (i IntMetric) String() string {
	return strconv.Itoa(int(i))
}

func (s *Server) RegisterWorker(w http.ResponseWriter, r *http.Request) {
	teamName := r.FormValue(":team_name")
	logger := s.logger.Session("register-team-worker", lager.Data{"team-name": teamName})

	savedTeam, found, err := s.teamDBFactory.GetTeamDB(teamName).GetTeam()
	if err != nil {
		logger.Error("failed-to-get-team", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !found {
		logger.Error("team-not-found", errors.New("team-not-found"))
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var registration atc.Worker
	err = json.NewDecoder(r.Body).Decode(&registration)
	if err != nil {
		logger.Error("failed-to-parse-worker-request", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if len(registration.GardenAddr) == 0 {
		logger.Error("missing-address", err)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "missing address")
		return
	}

	var ttl time.Duration

	ttlStr := r.URL.Query().Get("ttl")
	if len(ttlStr) > 0 {
		ttl, err = time.ParseDuration(ttlStr)
		if err != nil {
			logger.Error("malformed-ttl", err)
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

	_, err = s.teamDB.SaveWorker(db.WorkerInfo{
		GardenAddr:       registration.GardenAddr,
		BaggageclaimURL:  registration.BaggageclaimURL,
		HTTPProxyURL:     registration.HTTPProxyURL,
		HTTPSProxyURL:    registration.HTTPSProxyURL,
		NoProxy:          registration.NoProxy,
		ActiveContainers: registration.ActiveContainers,
		ResourceTypes:    registration.ResourceTypes,
		Platform:         registration.Platform,
		Tags:             registration.Tags,
		Team:             savedTeam.Name,
		Name:             registration.Name,
		StartTime:        registration.StartTime,
	}, ttl)
	if err != nil {
		logger.Error("failed-to-save-worker", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
