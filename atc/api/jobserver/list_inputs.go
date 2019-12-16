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
		w.Header().Set("Content-Type", "application/json")
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

		resources, err := pipeline.Resources()
		if err != nil {
			logger.Error("failed-to-get-resources", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		buildInputs, found, err := job.GetFullNextBuildInputs()
		if err != nil {
			logger.Error("failed-to-get-next-build-inputs", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		jobConfig, err := job.Config()
		if err != nil {
			logger.Error("failed-to-get-job-config", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		jobInputs := jobConfig.Inputs()

		inputs := make([]atc.BuildInput, len(buildInputs))

		for i, input := range buildInputs {
			var config atc.JobInputParams
			for _, jobInput := range jobInputs {
				if jobInput.Name == input.Name {
					config = jobInput
					break
				}
			}

			resource, found := resources.Lookup(config.Resource)
			if !found {
				logger.Debug("resource-is-not-found")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			inputs[i] = present.BuildInput(input, config, resource)
		}

		err = json.NewEncoder(w).Encode(inputs)
		if err != nil {
			logger.Error("failed-to-encode-build-inputs", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
