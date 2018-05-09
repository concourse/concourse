package volumeserver

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"code.cloudfoundry.org/lager"
)

// ReportWorkerVolumes provides an API endpoint for workers to report their current volumes
func (s *Server) ReportWorkerVolumes(w http.ResponseWriter, r *http.Request) {

	workerName := r.URL.Query().Get("worker_name")
	w.Header().Set("Content-Type", "application/json")

	logger := s.logger.Session("report-volumes-for-worker", lager.Data{"name": workerName})

	if workerName != "" {
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
			"handles": handles,
		})

		err = s.destroyer.DestroyVolumes(workerName, handles)
		if err != nil {
			logger.Error("failed-to-destroy-containers", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	} else {
		logger.Info("missing-worker-name")
		w.WriteHeader(http.StatusNotFound)
	}
}
