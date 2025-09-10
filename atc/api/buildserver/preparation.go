package buildserver

import (
	"net/http"

	"code.cloudfoundry.org/lager/v3"
	"github.com/bytedance/sonic"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) GetBuildPreparation(build db.BuildForAPI) http.Handler {
	logger := s.logger.Session("build-preparation", lager.Data{"build-id": build.ID()})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prep, found, err := build.Preparation()
		if err != nil {
			logger.Error("cannot-find-build-preparation", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		err = sonic.ConfigDefault.NewEncoder(w).Encode(present.BuildPreparation(prep))
		if err != nil {
			logger.Error("failed-to-encode-build-preparation", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
