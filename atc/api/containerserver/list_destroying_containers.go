package containerserver

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"
)

func (s *Server) ListDestroyingContainers(w http.ResponseWriter, r *http.Request) {

	workerName := r.URL.Query().Get("worker_name")
	w.Header().Set("Content-Type", "application/json")

	logger := s.logger.Session("list-destroying-containers-worker", lager.Data{"name": workerName})

	if workerName != "" {
		containerHandles, err := s.containerRepository.FindDestroyingContainers(workerName)
		if err != nil {
			logger.Error("failed-to-find-destroying-worker", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		logger.Debug("list", lager.Data{"destroying-container-count": len(containerHandles)})

		err = json.NewEncoder(w).Encode(containerHandles)
		if err != nil {
			logger.Error("failed-to-marshall-container-handles-for-worker", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	} else {
		logger.Info("failed-to-find-worker")
		w.WriteHeader(http.StatusNotFound)
	}
}
