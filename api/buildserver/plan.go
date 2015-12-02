package buildserver

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func (s *Server) GetBuildPlan(w http.ResponseWriter, r *http.Request) {
	hLog := s.logger.Session("get-build-plan")

	buildIDStr := r.FormValue(":build_id")

	buildID, err := strconv.Atoi(buildIDStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	build, found, err := s.db.GetBuild(buildID)
	if err != nil {
		s.logger.Error("failed-to-get-build", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	engineBuild, err := s.engine.LookupBuild(hLog, build)
	if err != nil {
		hLog.Error("failed-to-lookup-build", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	plan, found, err := engineBuild.PublicPlan(hLog)
	if err != nil {
		hLog.Error("failed-to-generate-plan", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(plan)
}
