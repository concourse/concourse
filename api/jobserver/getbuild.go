package jobserver

import (
	"encoding/json"
	"net/http"
)

func (s *Server) GetJobBuild(w http.ResponseWriter, r *http.Request) {
	jobName := r.FormValue(":job_name")
	buildName := r.FormValue(":build_name")

	build, err := s.db.GetJobBuild(jobName, buildName)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(build)
}
