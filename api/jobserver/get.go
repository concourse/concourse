package jobserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/web/routes"
	"github.com/tedsuo/rata"
)

func (s *Server) GetJob(w http.ResponseWriter, r *http.Request) {
	jobName := r.FormValue(":job_name")

	finished, next, err := s.db.GetJobFinishedAndNextBuild(jobName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	generator := rata.NewRequestGenerator("", routes.Routes)

	req, err := generator.CreateRequest(
		routes.GetJob,
		rata.Params{"job": jobName},
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

	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(atc.Job{
		Name:          jobName,
		URL:           req.URL.String(),
		FinishedBuild: finishedBuild,
		NextBuild:     nextBuild,
	})
}
