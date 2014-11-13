package jobserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
)

func (s *Server) ListJobs(w http.ResponseWriter, r *http.Request) {
	var jobs []atc.Job

	config, err := s.configDB.GetConfig()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, job := range config.Jobs {
		finished, next, err := s.db.GetJobFinishedAndNextBuild(job.Name)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		jobs = append(jobs, present.Job(job, config.Groups, finished, next))
	}

	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(jobs)
}
