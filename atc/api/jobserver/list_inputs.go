package jobserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) ListJobInputs(pipeline db.Pipeline) http.Handler {
	logger := s.logger.Session("list-job-inputs")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jobName := r.FormValue(":job_name")

		variables := s.variablesFactory.NewVariables(pipeline.TeamName(), pipeline.Name())

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

		scheduler := s.schedulerFactory.BuildScheduler(pipeline, s.externalURL, variables)

		resources, err := pipeline.Resources()
		if err != nil {
			logger.Error("failed-to-get-resources", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		err = scheduler.SaveNextInputMapping(logger, job, resources)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		buildInputs, found, err := job.GetNextBuildInputs()
		if err != nil {
			logger.Error("failed-to-get-next-build-inputs", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		jobInputs := job.Config().Inputs()
		presentedBuildInputs := make([]atc.BuildInput, len(buildInputs))
		for i, input := range buildInputs {
			var config atc.JobInput
			for _, jobInput := range jobInputs {
				if jobInput.Name == input.Name {
					config = jobInput
					break
				}
			}
			resource, _ := resources.Lookup(config.Resource)

			presentedBuildInputs[i] = present.BuildInput(input, config, resource)
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(presentedBuildInputs)
		if err != nil {
			logger.Error("failed-to-encode-build-inputs", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
