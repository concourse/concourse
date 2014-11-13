package jobserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/web/routes"
	"github.com/tedsuo/rata"
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

		generator := rata.NewRequestGenerator("", routes.Routes)

		req, err := generator.CreateRequest(
			routes.GetJob,
			rata.Params{"job": job.Name},
			nil,
		)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var nextBuild, finishedBuild *atc.Build

		if next != nil {
			presented := present.Build(*next)
			nextBuild = &presented
		}

		if finished != nil {
			presented := present.Build(*finished)
			finishedBuild = &presented
		}

		jobs = append(jobs, atc.Job{
			Name:          job.Name,
			URL:           req.URL.String(),
			FinishedBuild: finishedBuild,
			NextBuild:     nextBuild,
		})
	}

	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(jobs)
}
