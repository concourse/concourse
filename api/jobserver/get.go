package jobserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/api/resources"
)

func (s *Server) GetJob(w http.ResponseWriter, r *http.Request) {
	jobName := r.FormValue(":job_name")

	finished, next, err := s.db.GetJobFinishedAndNextBuild(jobName)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)

	var nextBuild, finishedBuild *resources.Build

	if next != nil {
		presented := present.Build(*next)
		nextBuild = &presented
	}

	if finished != nil {
		presented := present.Build(*finished)
		finishedBuild = &presented
	}

	json.NewEncoder(w).Encode(resources.Job{
		FinishedBuild: finishedBuild,
		NextBuild:     nextBuild,
	})
}
