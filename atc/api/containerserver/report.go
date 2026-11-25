package containerserver

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/concourse/concourse/v8/atc/db"
	"github.com/concourse/concourse/v8/atc/metric"

	"code.cloudfoundry.org/lager/v3"
)

func (s *Server) ReportWorkerContainers(worker db.Worker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		logger := s.logger.Session("report-containers-for-worker", lager.Data{"worker": worker.Name()})

		defer r.Body.Close()
		data, err := io.ReadAll(r.Body)
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

		numUnknownContainers, err := s.containerRepository.DestroyUnknownContainers(worker.Name(), handles)
		if err != nil {
			logger.Error("failed-to-destroy-unknown-containers", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if numUnknownContainers > 0 {
			logger.Info("unknown-container-handles", lager.Data{
				"handles-count": numUnknownContainers,
			})
		}

		metric.WorkerUnknownContainers{
			WorkerName: worker.Name(),
			Containers: numUnknownContainers,
		}.Emit(logger)

		err = s.containerRepository.UpdateContainersMissingSince(worker.Name(), handles)
		if err != nil {
			logger.Error("failed-to-update-containers-missing-since", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		err = s.destroyer.DestroyContainers(worker.Name(), handles)
		if err != nil {
			logger.Error("failed-to-destroy-containers", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
