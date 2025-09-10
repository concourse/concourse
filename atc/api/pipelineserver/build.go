package pipelineserver

import (
	"net/http"

	"code.cloudfoundry.org/lager/v3"
	"github.com/bytedance/sonic"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
)

func (s *Server) CreateBuild(pipeline db.Pipeline) http.Handler {
	logger := s.logger.Session("create-build")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var plan atc.Plan
		err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&plan)
		if err != nil {
			logger.Info("malformed-request", lager.Data{"error": err.Error()})
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		build, err := pipeline.CreateStartedBuild(plan)
		if err != nil {
			logger.Error("failed-to-create-one-off-build", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		err = sonic.ConfigDefault.NewEncoder(w).Encode(present.Build(build, nil, nil))
		if err != nil {
			logger.Error("failed-to-encode-build", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
