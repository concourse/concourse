package jobserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/db"
)

func (s *Server) ListJobBuilds(pipelineDB db.PipelineDB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jobName := r.FormValue(":job_name")

		builds, err := pipelineDB.GetAllJobBuilds(jobName)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusOK)

		resources := make([]atc.Build, len(builds))
		for i := 0; i < len(builds); i++ {
			resources[i] = present.Build(builds[i])
		}

		json.NewEncoder(w).Encode(resources)
	})
}
