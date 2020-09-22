package jobserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) GetJob(pipeline db.Pipeline) http.Handler {
	logger := s.logger.Session("get-job")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jobName := r.FormValue(":job_name")

		job, found, err := pipeline.Job(jobName)
		if err != nil {
			logger.Error("could-not-get-job-finished", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		inputs, err := job.Inputs()
		if err != nil {
			logger.Error("could-not-get-job-inputs", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		outputs, err := job.Outputs()
		if err != nil {
			logger.Error("could-not-get-job-inputs", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		finished, next, err := job.FinishedAndNextBuild()
		if err != nil {
			logger.Error("could-not-get-job-finished-and-next-build", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		err = json.NewEncoder(w).Encode(present.Job(
			job,
			inputs,
			outputs,
			finished,
			next,
			nil,
		))
		if err != nil {
			logger.Error("failed-to-encode-job", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
