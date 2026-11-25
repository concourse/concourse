package volumeserver

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/concourse/concourse/v8/atc/db"
	"github.com/concourse/concourse/v8/atc/metric"

	"code.cloudfoundry.org/lager/v3"
)

// ReportWorkerVolumes provides an API endpoint for workers to report their current volumes
func (s *Server) ReportWorkerVolumes(worker db.Worker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		logger := s.logger.Session("report-volumes-for-worker", lager.Data{"worker": worker.Name()})

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
			"handles-count": len(handles),
		})

		numUnknownVolumes, err := s.repository.DestroyUnknownVolumes(worker.Name(), handles)
		if err != nil {
			logger.Error("failed-to-destroy-unknown-volumes", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if numUnknownVolumes > 0 {
			logger.Info("unknown-volume-handles", lager.Data{
				"worker-name":   worker.Name(),
				"handles-count": numUnknownVolumes,
			})
		}

		metric.WorkerUnknownVolumes{
			WorkerName: worker.Name(),
			Volumes:    numUnknownVolumes,
		}.Emit(logger)

		err = s.repository.UpdateVolumesMissingSince(worker.Name(), handles)
		if err != nil {
			logger.Error("failed-to-update-volumes-missing-since", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		err = s.destroyer.DestroyVolumes(worker.Name(), handles)
		if err != nil {
			logger.Error("failed-to-destroy-volumes", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
