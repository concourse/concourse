package buildserver

import (
	"encoding/json"
	"net/http"
)

func (s *Server) ListBuilds(w http.ResponseWriter, r *http.Request) {
	builds, err := s.db.GetAllBuilds()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(builds)
}
