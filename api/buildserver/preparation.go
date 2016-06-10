package buildserver

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/concourse/atc/api/present"
	"github.com/pivotal-golang/lager"
)

func (s *Server) GetBuildPreparation(w http.ResponseWriter, r *http.Request) {
	buildIDStr := r.FormValue(":build_id")
	log := s.logger.Session("build-preparation", lager.Data{"build-id": buildIDStr})

	buildID, err := strconv.Atoi(buildIDStr)
	if err != nil {
		log.Error("cannot-parse-build-id", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	teamDB := s.teamDBFactory.GetTeamDB(getTeamName(r))
	build, found, err := teamDB.GetBuild(buildID)
	if err != nil {
		s.logger.Error("failed-to-get-build", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	prep, found, err := build.GetPreparation()
	if err != nil {
		log.Error("cannot-find-build-preparation", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(present.BuildPreparation(prep))
}
