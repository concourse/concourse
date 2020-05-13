package jobserver

import (
	"context"
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) CreateJobBuild(pipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		logger := s.logger.Session("create-job-build")

		jobName := r.FormValue(":job_name")

		job, found, err := pipeline.Job(jobName)
		if err != nil {
			logger.Error("failed-to-get-resource-types", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if job.DisableManualTrigger() {
			w.WriteHeader(http.StatusConflict)
			return
		}

		build, err := job.CreateBuild()
		if err != nil {
			logger.Error("failed-to-create-job-build", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		resources, err := pipeline.Resources()
		if err != nil {
			logger.Error("failed-to-get-resources", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		resourceTypes, err := pipeline.ResourceTypes()
		if err != nil {
			logger.Error("failed-to-get-resource-types", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		inputs, err := job.Inputs()
		if err != nil {
			logger.Error("failed-to-get-job-inputs", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		for _, input := range inputs {
			resource, found := resources.Lookup(input.Resource)
			if found {
				version := resource.CurrentPinnedVersion()
				_, _, err := s.checkFactory.TryCreateCheck(
					lagerctx.NewContext(context.Background(), logger),
					resource,
					resourceTypes,
					version,
					true,
				)
				if err != nil {
					logger.Error("failed-to-create-check", err)
				}
			}
		}

		err = s.checkFactory.NotifyChecker()
		if err != nil {
			logger.Error("failed-to-notify-checker", err)
		}

		err = json.NewEncoder(w).Encode(present.Build(build))
		if err != nil {
			logger.Error("failed-to-encode-build", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
