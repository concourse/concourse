package jobserver

import (
	"net/http"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
	"github.com/tedsuo/rata"
)

func (s *Server) PauseJob(pipelineDB db.PipelineDB, _ dbng.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jobName := rata.Param(r, "job_name")

		err := pipelineDB.PauseJob(jobName)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
