package jobserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
)

func (s *Server) ListJobBuilds(w http.ResponseWriter, r *http.Request) {
	jobName := r.FormValue(":job_name")

	builds, err := s.db.GetAllJobBuilds(jobName)
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
}
