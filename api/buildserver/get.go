package buildserver

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
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

	dbBuild, found, err := s.db.GetBuild(buildID)
	if err != nil {
		log.Error("cannot-find-build", err, lager.Data{"buildID": r.FormValue(":build_id")})
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)

	dbBuild.TeamName = atc.DefaultTeamName
	build := present.Build(dbBuild)

	json.NewEncoder(w).Encode(build)
}
