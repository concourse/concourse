package containerserver

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/concourse/concourse/atc/metric"

	"code.cloudfoundry.org/lager"
)

func (s *Server) ReportWorkerContainers(w http.ResponseWriter, r *http.Request) {
	workerName := r.URL.Query().Get("worker_name")
	w.Header().Set("Content-Type", "application/json")

	logger := s.logger.Session("report-containers-for-worker", lager.Data{"name": workerName})

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
		"num-handles": len(handles),
	})

	numUnknownContainers, err := s.containerRepository.DestroyUnknownContainers(workerName, handles)
	if err != nil {
		logger.Error("failed-to-destroy-unknown-containers", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if numUnknownContainers > 0 {
		logger.Info("unknown-container-handles", lager.Data{
			"worker-name":   workerName,
			"handles-count": numUnknownContainers,
		})
	}

	metric.WorkerUnknownContainers{
		WorkerName: workerName,
		Containers: numUnknownContainers,
	}.Emit(logger)

	err = s.containerRepository.UpdateContainersMissingSince(workerName, handles)
	if err != nil {
		logger.Error("failed-to-update-containers-missing-since", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = s.destroyer.DestroyContainers(workerName, handles)
	if err != nil {
		logger.Error("failed-to-destroy-containers", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
