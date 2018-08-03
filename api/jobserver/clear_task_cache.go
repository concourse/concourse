package jobserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

func (s *Server) ClearTaskCache(pipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session("clear-task-cache")
		jobName := r.FormValue(":job_name")
		stepName := r.FormValue(":step_name")
		cachePath := r.FormValue(atc.ClearTaskCacheQueryPath)

		job, found, err := pipeline.Job(jobName)
		if err != nil {
			logger.Error("failed-to-get-job", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			logger.Debug("could-not-find-job", lager.Data{
				"jobName":   jobName,
				"stepName":  stepName,
				"cachePath": cachePath,
			})
			w.WriteHeader(http.StatusNotFound)
			return
		}

		rowsDeleted, err := job.ClearTaskCache(stepName, cachePath)

		if rowsDeleted == 0 {
			logger.Debug("could-not-find-cache-path", lager.Data{
				"jobName":   jobName,
				"stepName":  stepName,
				"cachePath": cachePath,
			})
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err != nil {
			logger.Error("failed-to-clear-task-cache", err)
			w.WriteHeader(http.StatusInternalServerError)
			s.writeResponse(w, []string{err.Error()})
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

func (s *Server) writeResponse(w http.ResponseWriter, messages []string) {
	w.Header().Set("Content-Type", "application/json")

	responseJSON, err := json.Marshal(messages)
	if err != nil {
		s.logger.Error("failed-to-marshal-response", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to generate error response: %s", err)
		return
	}

	_, err = w.Write(responseJSON)
	if err != nil {
		s.logger.Error("failed-to-write-response", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
