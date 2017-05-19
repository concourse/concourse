package jobserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
)

func (s *Server) ListJobInputs(_ db.PipelineDB, pipeline dbng.Pipeline) http.Handler {
	logger := s.logger.Session("list-job-inputs")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jobName := r.FormValue(":job_name")

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

		scheduler := s.schedulerFactory.BuildScheduler(pipeline, s.externalURL)

		err = scheduler.SaveNextInputMapping(logger, job.Config())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		buildInputs, found, err := pipeline.GetNextBuildInputs(jobName)
		if err != nil {
			logger.Error("failed-to-get-next-build-inputs", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		resources, err := pipeline.Resources()
		if err != nil {
			logger.Error("failed-to-get-resources", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		jobInputs := job.Config().Inputs()
		presentedBuildInputs := make([]atc.BuildInput, len(buildInputs))
		for i, input := range buildInputs {
			resource, _ := resources.Lookup(input.Resource)

			var config atc.JobInput
			for _, jobInput := range jobInputs {
				if jobInput.Name == input.Name {
					config = jobInput
					break
				}
			}

			presentedBuildInputs[i] = present.BuildInput(input, config, resource.Source())
		}

		json.NewEncoder(w).Encode(presentedBuildInputs)
	})
}
