package jobserver

import (
	"net/http"

	"github.com/concourse/atc/db"
	"github.com/tedsuo/rata"
)

func (s *Server) UnpauseJob(pipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session("unpause-job")
		jobName := rata.Param(r, "job_name")

		job, found, err := pipeline.Job(jobName)
		if err != nil {
			logger.Error("failed-to-get-job", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		err = job.Unpause()
		if err != nil {
			logger.Error("failed-to-unpause-job", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
