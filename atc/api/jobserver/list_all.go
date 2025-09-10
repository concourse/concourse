package jobserver

import (
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor"
)

func (s *Server) ListAllJobs(w http.ResponseWriter, r *http.Request) {
	logger := s.logger.Session("list-all-jobs")

	acc := accessor.GetAccessor(r)

	var jobs []atc.JobSummary
	var err error
	if acc.IsAdmin() {
		jobs, err = s.jobFactory.AllActiveJobs()
	} else {
		jobs, err = s.jobFactory.VisibleJobs(acc.TeamNames())
	}

	if err != nil {
		logger.Error("failed-to-get-all-visible-jobs", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if jobs == nil {
		jobs = []atc.JobSummary{}
	}

	w.Header().Set("Content-Type", "application/json")
	err = sonic.ConfigDefault.NewEncoder(w).Encode(jobs)
	if err != nil {
		logger.Error("failed-to-encode-jobs", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
