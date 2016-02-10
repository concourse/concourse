package buildserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc/api/present"
	"github.com/pivotal-golang/lager"
)

func (s *Server) GetBuildPreparation(w http.ResponseWriter, r *http.Request) {
	log := s.logger.Session("build-preparation")

	prep, found, err := s.db.GetBuildPreparation(4)
	if err != nil {
		log.Error("cannot-find-build-preparation", err, lager.Data{"buildID": r.FormValue(":build_id")})
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(present.BuildPreparation(prep))
}
