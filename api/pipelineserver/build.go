package pipelineserver

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/db"
)

func (s *Server) CreateBuild(pipelineDB db.Pipeline) http.Handler {
	logger := s.logger.Session("create-build")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var plan atc.Plan
		err := json.NewDecoder(r.Body).Decode(&plan)
		if err != nil {
			logger.Info("malformed-request", lager.Data{"error": err.Error()})
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		build, err := pipelineDB.CreateOneOffBuild()
		if err != nil {
			logger.Error("failed-to-create-one-off-build", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		engineBuild, err := s.engine.CreateBuild(logger, build, plan)
		if err != nil {
			logger.Error("failed-to-start-build", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		go engineBuild.Resume(logger)

		w.WriteHeader(http.StatusCreated)

		err = json.NewEncoder(w).Encode(present.Build(build))
		if err != nil {
			logger.Error("failed-to-encode-build", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
