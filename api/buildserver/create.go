package buildserver

import (
	"encoding/json"
	"net/http"

	tbuilds "github.com/concourse/turbine/api/builds"
)

func (s *Server) CreateBuild(w http.ResponseWriter, r *http.Request) {
	var turbineBuild tbuilds.Build
	err := json.NewDecoder(r.Body).Decode(&turbineBuild)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	build, err := s.db.CreateOneOffBuild()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = s.builder.Build(build, turbineBuild)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(build)
}
