package jobserver

import (
	"encoding/json"
	"net/http"
)

func (s *Server) GetJobCurrentBuild(w http.ResponseWriter, r *http.Request) {
	jobName := r.FormValue(":job_name")

	build, err := s.db.GetCurrentBuild(jobName)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(build)
}
