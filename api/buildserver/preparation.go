package buildserver

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/dbng"
)

func (s *Server) GetBuildPreparation(build dbng.Build) http.Handler {
	log := s.logger.Session("build-preparation", lager.Data{"build-id": build.ID()})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prep, found, err := build.Preparation()
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
	})
}
