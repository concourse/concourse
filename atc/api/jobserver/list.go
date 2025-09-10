package jobserver

import (
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) ListJobs(pipeline db.Pipeline) http.Handler {
	logger := s.logger.Session("list-jobs")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jobs, err := pipeline.Dashboard()
		if err != nil {
			logger.Error("failed-to-get-dashboard", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if jobs == nil {
			jobs = []atc.JobSummary{}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		err = sonic.ConfigDefault.NewEncoder(w).Encode(jobs)
		if err != nil {
			logger.Error("failed-to-encode-jobs", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
