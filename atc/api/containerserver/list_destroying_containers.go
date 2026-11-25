package containerserver

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/v8/atc/db"
)

func (s *Server) ListDestroyingContainers(worker db.Worker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session("list-destroying-containers", lager.Data{"worker": worker.Name()})

		containerHandles, err := s.containerRepository.FindDestroyingContainers(worker.Name())
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
	})
}
