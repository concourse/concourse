package jobserver

import (
	"encoding/json"
	"net/http"
)

func (s *Server) ListJobBuilds(w http.ResponseWriter, r *http.Request) {
	jobName := r.FormValue(":job_name")

	builds, err := s.db.GetAllJobBuilds(jobName)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(builds)
}
