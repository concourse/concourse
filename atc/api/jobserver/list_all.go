package jobserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/present"
)

func (s *Server) ListAllJobs(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("list-all-jobs")
	if s.disableListAllJobs {
		logger.Debug("endpoint-disabled")
		err := json.NewEncoder(w).Encode([]atc.Job{})
		if err != nil {
			logger.Error("failed-to-encode-jobs", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	acc := accessor.GetAccessor(r)

	dashboard, err := s.jobFactory.VisibleJobs(acc.TeamNames())
	if err != nil {
		logger.Error("failed-to-get-all-visible-jobs", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	jobs := []atc.Job{}

	for _, job := range dashboard {
		jobs = append(
			jobs,
			present.Job(
				job.Job.TeamName(),
				job.Job,
				job.FinishedBuild,
				job.NextBuild,
				job.TransitionBuild,
			),
		)
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(jobs)
	if err != nil {
		logger.Error("failed-to-encode-jobs", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
