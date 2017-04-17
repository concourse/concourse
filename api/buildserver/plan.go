package buildserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc/dbng"
)

func (s *Server) GetBuildPlan(build dbng.Build) http.Handler {
	hLog := s.logger.Session("get-build-plan")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		engineBuild, err := s.engine.LookupBuild(hLog, build)
		if err != nil {
			hLog.Error("failed-to-lookup-build", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		plan, err := engineBuild.PublicPlan(hLog)
		if err != nil {
			hLog.Error("failed-to-generate-plan", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(plan)
	})
}
