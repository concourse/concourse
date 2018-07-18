package volumeserver

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/metric"
)

func (s *Server) ListDestroyingVolumes(w http.ResponseWriter, r *http.Request) {
	workerName := r.URL.Query().Get("worker_name")
	w.Header().Set("Content-Type", "application/json")

	logger := s.logger.Session("marked-volumes-for-worker", lager.Data{"worker_name": workerName})

	if workerName != "" {
		destroyingVolumesHandles, err := s.destroyer.FindDestroyingVolumesForGc(workerName)
		if err != nil {
			logger.Error("failed-to-find-destroying-volumes", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		logger.Debug("list", lager.Data{"destroying-volume-count": len(destroyingVolumesHandles)})

		metric.VolumesToBeGarbageCollected{
			Volumes: len(destroyingVolumesHandles),
		}.Emit(logger)

		err = json.NewEncoder(w).Encode(destroyingVolumesHandles)
		if err != nil {
			logger.Error("failed-to-encode-volumes", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	} else {
		logger.Info("failed-to-find-worker")
		w.WriteHeader(http.StatusNotFound)
	}
}
