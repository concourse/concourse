package jobserver

import (
	"net/http"

	"github.com/tedsuo/rata"
)

func (s *Server) UnpauseJob(w http.ResponseWriter, r *http.Request) {
	jobName := rata.Param(r, "job_name")

	err := s.db.UnpauseJob(jobName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
