package jobserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/db"
)

func (s *Server) GetJob(pipelineDB db.PipelineDB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jobName := r.FormValue(":job_name")

		config, _, err := pipelineDB.GetConfig()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		job, found := config.Jobs.Lookup(jobName)
		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		finished, next, err := pipelineDB.GetJobFinishedAndNextBuild(jobName)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		dbJob, err := pipelineDB.GetJob(job.Name)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)

		json.NewEncoder(w).Encode(present.Job(dbJob, job, config.Groups, finished, next))
	})
}
