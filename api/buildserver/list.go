package buildserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
)

func (s *Server) ListBuilds(w http.ResponseWriter, r *http.Request) {
	builds, err := s.db.GetAllBuilds()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)

	atc := make([]atc.Build, len(builds))
	for i := 0; i < len(builds); i++ {
		atc[i] = present.Build(builds[i])
	}

	json.NewEncoder(w).Encode(atc)
}
