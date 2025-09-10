package buildserver

import (
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) GetBuildPlan(build db.BuildForAPI) http.Handler {
	hLog := s.logger.Session("get-build-plan")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !build.HasPlan() {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		err := sonic.ConfigDefault.NewEncoder(w).Encode(atc.PublicBuildPlan{
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
