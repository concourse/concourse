package volumeserver

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/v8/atc/db"
	"github.com/concourse/concourse/v8/atc/metric"
)

func (s *Server) ListDestroyingVolumes(worker db.Worker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session("list-destroying-volumes", lager.Data{"worker": worker.Name()})

		volumeHandles, err := s.destroyer.FindDestroyingVolumesForGc(worker.Name())
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
	})
}
