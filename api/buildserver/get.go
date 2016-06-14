package buildserver

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/auth"
	"github.com/pivotal-golang/lager"
)

func (s *Server) GetBuild(w http.ResponseWriter, r *http.Request) {
	log := s.logger.Session("build-resources")
	buildID, err := strconv.Atoi(r.FormValue(":build_id"))
	if err != nil {
		log.Error("cannot-parse-build-id", err, lager.Data{"buildID": r.FormValue(":build_id")})
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	teamDB := s.teamDBFactory.GetTeamDB(auth.GetAuthOrDefaultTeamName(r))
	build, found, err := teamDB.GetBuild(buildID)
	if err != nil {
		log.Error("cannot-find-build", err, lager.Data{"buildID": r.FormValue(":build_id")})
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if found {
		hasAccess, err := s.verifyBuildAcccess(build, r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !hasAccess {
			s.rejector.Unauthorized(w, r)
			return
		}
	} else {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(present.Build(build))
}
