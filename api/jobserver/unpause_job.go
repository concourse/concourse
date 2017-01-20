package jobserver

import (
	"net/http"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
	"github.com/tedsuo/rata"
)

func (s *Server) UnpauseJob(pipelineDB db.PipelineDB, _ dbng.Pipeline) http.Handler {
	logger := s.logger.Session("unpause-job")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jobName := rata.Param(r, "job_name")

		err := pipelineDB.UnpauseJob(jobName)
		if err != nil {
			logger.Error("failed-to-unpause-job", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
