package jobserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/db"
)

func (s *Server) ListJobInputs(pipelineDB db.PipelineDB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jobName := r.FormValue(":job_name")

		config, _, err := pipelineDB.GetConfig()
		switch err {
		case db.ErrPipelineNotFound:
			w.WriteHeader(http.StatusNotFound)
			return
		case nil:
		default:
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		jobConfig, found := config.Jobs.Lookup(jobName)
		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		versionsDB, err := pipelineDB.LoadVersionsDB()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		jobInputs := jobConfig.Inputs()

		inputVersions, err := pipelineDB.GetLatestInputVersions(versionsDB, jobName, jobInputs)
		switch err {
		case db.ErrNoVersions:
			w.WriteHeader(http.StatusNotFound)
			return
		case nil:
		default:
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		buildInputs := make([]atc.BuildInput, len(inputVersions))
		for i, input := range inputVersions {
			resource, _ := config.Resources.Lookup(input.Resource)
			buildInputs[i] = present.BuildInput(input, resource.Source)
		}

		json.NewEncoder(w).Encode(buildInputs)
	})
}
