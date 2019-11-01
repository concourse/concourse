package volumeserver

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/concourse/concourse/atc/metric"

	"code.cloudfoundry.org/lager"
)

// ReportWorkerVolumes provides an API endpoint for workers to report their current volumes
func (s *Server) ReportWorkerVolumes(w http.ResponseWriter, r *http.Request) {
	workerName := r.URL.Query().Get("worker_name")
	w.Header().Set("Content-Type", "application/json")

	logger := s.logger.Session("report-volumes-for-worker", lager.Data{"name": workerName})

	if workerName == "" {
		logger.Info("missing-worker-name")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	defer r.Body.Close()

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logger.Error("failed-to-read-body", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var handles []string
	err = json.Unmarshal(data, &handles)
	if err != nil {
		logger.Error("failed-to-unmarshal-body", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	logger.Debug("handles-info", lager.Data{
		"handles-count": len(handles),
	})

	numUnknownVolumes, err := s.repository.DestroyUnknownVolumes(workerName, handles)
	if err != nil {
		logger.Error("failed-to-destroy-unknown-volumes", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if numUnknownVolumes > 0 {
		logger.Info("unknown-volume-handles", lager.Data{
			"worker-name":   workerName,
			"handles-count": numUnknownVolumes,
		})
	}

	metric.WorkerUnknownVolumes{
		WorkerName: workerName,
		Volumes:    numUnknownVolumes,
	}.Emit(logger)

	err = s.repository.UpdateVolumesMissingSince(workerName, handles)
	if err != nil {
		logger.Error("failed-to-update-volumes-missing-since", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = s.destroyer.DestroyVolumes(workerName, handles)
	if err != nil {
		logger.Error("failed-to-destroy-volumes", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
