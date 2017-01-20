package jobserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
)

func (s *Server) ListJobInputs(pipelineDB db.PipelineDB, dbPipeline dbng.Pipeline) http.Handler {
	logger := s.logger.Session("list-job-inputs")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jobName := r.FormValue(":job_name")

		pipelineConfig := pipelineDB.Config()

		jobConfig, found := pipelineConfig.Jobs.Lookup(jobName)
		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		scheduler := s.schedulerFactory.BuildScheduler(pipelineDB, dbPipeline, s.externalURL)

		err := scheduler.SaveNextInputMapping(logger, jobConfig)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		buildInputs, found, err := pipelineDB.GetNextBuildInputs(jobName)
		if err != nil {
			logger.Error("failed-to-get-next-build-inputs", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		jobInputs := config.JobInputs(jobConfig)
		presentedBuildInputs := make([]atc.BuildInput, len(buildInputs))
		for i, input := range buildInputs {
			resource, _ := pipelineConfig.Resources.Lookup(input.Resource)

			var config config.JobInput
			for _, jobInput := range jobInputs {
				if jobInput.Name == input.Name {
					config = jobInput
					break
				}
			}

			presentedBuildInputs[i] = present.BuildInput(input, config, resource.Source)
		}

		json.NewEncoder(w).Encode(presentedBuildInputs)
	})
}
