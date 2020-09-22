package jobserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) ListJobs(pipeline db.Pipeline) http.Handler {
	logger := s.logger.Session("list-jobs")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jobs := []atc.Job{}

		dashboard, err := pipeline.Dashboard()

		if err != nil {
			logger.Error("failed-to-get-dashboard", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		for _, job := range dashboard {
			jobs = append(
				jobs,
				present.DashboardJob(job),
			)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		err = json.NewEncoder(w).Encode(jobs)
		if err != nil {
			logger.Error("failed-to-encode-jobs", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
