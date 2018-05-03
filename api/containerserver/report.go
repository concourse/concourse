package containerserver

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"code.cloudfoundry.org/lager"
)

func (s *Server) ReportWorkerContainers(w http.ResponseWriter, r *http.Request) {

	workerName := r.URL.Query().Get("worker_name")
	w.Header().Set("Content-Type", "application/json")

	logger := s.logger.Session("marked-containers-for-worker", lager.Data{"worker_name": workerName})

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

		err = s.containerDestroyer.Destroy(workerName, handles)
		if err != nil {
			logger.Error("failed-to-destroy-containers", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	} else {
		logger.Info("failed-to-find-worker")
		w.WriteHeader(http.StatusNotFound)
	}
}
