package buildserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/turbine"

	"github.com/concourse/atc/api/present"
)

func (s *Server) CreateBuild(w http.ResponseWriter, r *http.Request) {
	var turbineBuild turbine.Build
	err := json.NewDecoder(r.Body).Decode(&turbineBuild)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	build, err := s.db.CreateOneOffBuild()
	if err != nil {
		s.logger.Error("failed-to-create-one-off-build", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = s.builder.Build(build, turbineBuild)
	if err != nil {
		s.logger.Error("failed-to-start-build", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(present.Build(build))
}
