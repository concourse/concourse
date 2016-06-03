package jobserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/db"
)

func (s *Server) ListJobs(pipelineDB db.PipelineDB) http.Handler {
	logger := s.logger.Session("list-jobs")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var jobs []atc.Job

		dashboard, groups, err := pipelineDB.GetDashboard()
		if err != nil {
			logger.Error("failed-to-get-dashboard", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		teamName := r.FormValue(":team_name")

		for _, job := range dashboard {
			jobs = append(
				jobs,
				present.Job(
					teamName,
					job.Job,
					job.JobConfig,
					groups,
					job.FinishedBuild,
					job.NextBuild,
				),
			)
		}

		w.WriteHeader(http.StatusOK)

		json.NewEncoder(w).Encode(jobs)
	})
}
