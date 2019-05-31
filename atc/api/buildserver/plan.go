package buildserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/concourse/v5/atc"
	"github.com/concourse/concourse/v5/atc/db"
)

func (s *Server) GetBuildPlan(build db.Build) http.Handler {
	hLog := s.logger.Session("get-build-plan")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(atc.PublicBuildPlan{
			Schema: build.Schema(),
			Plan:   build.PublicPlan(),
		})
		if err != nil {
			hLog.Error("failed-to-encode-public-build-plan", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})
}
