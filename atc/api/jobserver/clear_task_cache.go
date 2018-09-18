package jobserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/google/jsonapi"
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
			w.Header().Set("Content-Type", jsonapi.MediaType)
			w.WriteHeader(http.StatusNotFound)
			jsonapi.MarshalErrors(w, []*jsonapi.ErrorObject{{
				Title:  "Job Not Found Error",
				Detail: fmt.Sprintf("Job with name '%s' not found.", jobName),
				Status: "404",
			}})
			return
		}

		rowsDeleted, err := job.ClearTaskCache(stepName, cachePath)

		if err != nil {
			logger.Error("failed-to-clear-task-cache", err)
			w.Header().Set("Content-Type", jsonapi.MediaType)
			w.WriteHeader(http.StatusInternalServerError)
			jsonapi.MarshalErrors(w, []*jsonapi.ErrorObject{{
				Title:  "Clear Task Cache Error",
				Detail: err.Error(),
				Status: "500",
			}})
			return
		}

		s.writeJSONResponse(w, atc.ClearTaskCacheResponse{CachesRemoved: rowsDeleted})
	})
}

func (s *Server) writeJSONResponse(w http.ResponseWriter, obj interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	responseJSON, err := json.Marshal(obj)
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
