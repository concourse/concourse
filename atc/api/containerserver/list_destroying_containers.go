package containerserver

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"
)

func (s *Server) ListDestroyingContainers(w http.ResponseWriter, r *http.Request) {
	workerName := r.URL.Query().Get("worker_name")

	logger := s.logger.Session("list-destroying-containers", lager.Data{"worker": workerName})

	if workerName == "" {
		logger.Info("no-worker-specified")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	containerHandles, err := s.containerRepository.FindDestroyingContainers(workerName)
	if err != nil {
		logger.Error("failed-to-find-destroying-worker", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	logger.Debug("containers-to-destroy", lager.Data{"count": len(containerHandles)})

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(containerHandles)
	if err != nil {
		logger.Error("failed-to-marshall-container-handles-for-worker", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
