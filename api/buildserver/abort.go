package buildserver

import (
	"net/http"
	"strconv"

	"github.com/pivotal-golang/lager"
)

func (s *Server) AbortBuild(w http.ResponseWriter, r *http.Request) {
	buildID, err := strconv.Atoi(r.FormValue(":build_id"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	aLog := s.logger.Session("abort", lager.Data{
		"build": buildID,
	})

	build, err := s.db.GetBuild(buildID)
	if err != nil {
		aLog.Error("failed-to-get-build", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	engineBuild, err := s.engine.LookupBuild(build)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = engineBuild.Abort()
	if err != nil {
		aLog.Error("failed-to-unmarshal-metadata", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
