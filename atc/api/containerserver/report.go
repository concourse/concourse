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

	logger := s.logger.Session("report-containers-for-worker", lager.Data{"name": workerName})

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
			"num-handles": len(handles),
		})

		err = s.destroyer.DestroyContainers(workerName, handles)
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
