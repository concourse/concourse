package jobserver

import (
	"net/http"

	"github.com/tedsuo/rata"
)

func (s *Server) PauseJob(w http.ResponseWriter, r *http.Request) {
	jobName := rata.Param(r, "job_name")

	err := s.db.PauseJob(jobName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
