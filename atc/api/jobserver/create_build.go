package jobserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) CreateJobBuild(pipeline db.Pipeline) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		logger := s.logger.Session("create-job-build")

		jobName := r.FormValue(":job_name")

		job, found, err := pipeline.Job(jobName)
		if s.checkErrorAndLogMessage(err, logger, w, "failed-to-get-resource-types", http.StatusInternalServerError) {
			return
		}

		if s.checkResultAndRespond(!found, w, http.StatusNotFound) {
			return
		}

		if s.checkResultAndRespond(job.DisableManualTrigger(), w, http.StatusConflict) {
			return
		}

		build, err := job.CreateBuild()
		if s.checkErrorAndLogMessage(err, logger, w, "failed-to-create-job-build", http.StatusInternalServerError) {
			return
		}

		resources, err := pipeline.Resources()

		if s.checkErrorAndLogMessage(err, logger, w, "failed-to-get-resources", http.StatusInternalServerError) {
			return
		}

		resourceTypes, err := pipeline.ResourceTypes()
		if s.checkErrorAndLogMessage(err, logger, w, "failed-to-get-resource-types", http.StatusInternalServerError) {
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
				_, _, err := s.checkFactory.TryCreateCheck(logger, resource, resourceTypes, version, true)
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
		s.checkErrorAndLogMessage(err, logger, w, "failed-to-encode-build", http.StatusInternalServerError)
	})
}
