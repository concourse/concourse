package volumeserver

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/metric"
)

func (s *Server) ListDestroyingVolumes(w http.ResponseWriter, r *http.Request) {
	workerName := r.URL.Query().Get("worker_name")

	logger := s.logger.Session("list-destroying-volumes", lager.Data{"worker": workerName})

	if workerName == "" {
		logger.Info("no-worker-specified")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	volumeHandles, err := s.destroyer.FindDestroyingVolumesForGc(workerName)
	if err != nil {
		logger.Error("failed-to-find-destroying-volumes", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	logger.Debug("volumes-to-destroy", lager.Data{"count": len(volumeHandles)})

	metric.VolumesToBeGarbageCollected{
		Volumes: len(volumeHandles),
	}.Emit(logger)

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(volumeHandles)
	if err != nil {
		logger.Error("failed-to-encode-volumes", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
