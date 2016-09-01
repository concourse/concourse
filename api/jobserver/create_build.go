package jobserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/db"
)

func (s *Server) CreateJobBuild(pipelineDB db.PipelineDB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.logger.Session("create-job-build")

		jobName := r.FormValue(":job_name")

		config := pipelineDB.Config()

		job, found := config.Jobs.Lookup(jobName)
		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if job.DisableManualTrigger {
			w.WriteHeader(http.StatusConflict)
			return
		}

		scheduler := s.schedulerFactory.BuildScheduler(pipelineDB, s.externalURL)

		build, _, err := scheduler.TriggerImmediately(logger, job, config.Resources, config.ResourceTypes)
		if err != nil {
			logger.Error("failed-to-trigger", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "failed to trigger: %s", err)
			return
		}

		json.NewEncoder(w).Encode(present.Build(build))
	})
}
