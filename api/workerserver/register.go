package workerserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/concourse/atc"
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

	metric.WorkerContainers{
		WorkerAddr: registration.GardenAddr,
		Containers: registration.ActiveContainers,
	}.Emit(s.logger)

	if registration.Name == "" {
		registration.Name = registration.GardenAddr
	}

	_, err = s.db.SaveWorker(db.WorkerInfo{
		GardenAddr:       registration.GardenAddr,
		BaggageclaimURL:  registration.BaggageclaimURL,
		HTTPProxyURL:     registration.HTTPProxyURL,
		HTTPSProxyURL:    registration.HTTPSProxyURL,
		NoProxy:          registration.NoProxy,
		ActiveContainers: registration.ActiveContainers,
		ResourceTypes:    registration.ResourceTypes,
		Platform:         registration.Platform,
		Tags:             registration.Tags,
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
