package configserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"

	"github.com/concourse/concourse/atc"
)

func (s *Server) GetConfig(pipeline db.Pipeline) http.Handler {
	logger := s.logger.Session("get-config")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if pipeline.Archived() {
			logger.Debug("pipeline-is-archived", lager.Data{"pipeline": pipeline.Name()})
			w.WriteHeader(http.StatusNotFound)
			return
		}

		config, err := pipeline.Config()
		if err != nil {
			logger.Error("failed-to-get-pipeline-config", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set(atc.ConfigVersionHeader, fmt.Sprintf("%d", pipeline.ConfigVersion()))
		w.Header().Set("Content-Type", "application/json")

		err = json.NewEncoder(w).Encode(atc.ConfigResponse{
			Config: config,
		})
		if err != nil {
			logger.Error("failed-to-encode-config", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}
